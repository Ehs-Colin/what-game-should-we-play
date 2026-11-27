package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/importer"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
	"github.com/jackc/pgx/v5/pgconn"
)

// maxSteamUserNameLength matches the steam_users.name column width.
const maxSteamUserNameLength = 255

const steamUsernameTakenMessage = "That username is already used by another user. Choose a different one."

var (
	errSteamUsernameTaken = errors.New("custom username already taken")
	errSteamNameTooLong   = errors.New("custom username too long")
)

type insertSteamUserRequest struct {
	// VanityUrl is a Steam custom URL name. When present, it is the
	// identifier and its resolution wins over any custom label.
	VanityUrl string `json:"vanityUrl"`
	// SteamID is the ID the Import page is showing. It's required when no
	// vanity name was entered and guards a vanity resolution from importing
	// a reassigned account.
	SteamID string `json:"steamId"`
	// CustomUsername labels a user imported by Steam ID who has no vanity.
	// It is ignored on the vanity path.
	CustomUsername string `json:"customUsername"`
	// Force skips the refresh-interval shortcut and always starts a new job
	// when none is active.
	Force bool `json:"force"`
}

// ImportRequester creates or returns a user's import job and reports their
// import state. It's satisfied by *importer.Importer.
type ImportRequester interface {
	RequestImport(ctx context.Context, steamID int64) (dbgen.ImportJob, bool, error)
	RequestForcedImport(ctx context.Context, steamID int64) (dbgen.ImportJob, bool, error)
	Snapshot(ctx context.Context, steamID int64) (importer.ImportSnapshot, error)
}

type insertSteamUserResponse struct {
	SteamUser steamUserView `json:"steamUser"`
	ImportJob importJobView `json:"importJob"`
}

func importJobLocation(id int64) string {
	return fmt.Sprintf("/api/import-jobs/%d", id)
}

// InsertSteamUser saves the user and queues a library import. A non-empty
// vanityUrl takes the vanity resolution path; a steamId alone skips the
// vanity step entirely and validates the ID through a read-only profile
// fetch. Either way the stored profile feeds the same import request: 202
// with a Location header when a new job was created, or 200 with the
// existing active or still-fresh job. Conflicts are rejected before anything
// is written: a steamId that no longer matches the resolved name (409) and a
// custom username already used by another user (409).
func (handler *Handler) InsertSteamUser(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO::Require user login with write permissions
	var body insertSteamUserRequest
	request.Body = http.MaxBytesReader(responseWriter, request.Body, 1<<10)
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "invalid JSON body")
		return
	}
	vanityUrl := strings.TrimSpace(body.VanityUrl)
	rawSteamID := strings.TrimSpace(body.SteamID)
	if vanityUrl == "" && rawSteamID == "" {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "vanityUrl or steamId is required")
		return
	}
	var enteredID int64
	if rawSteamID != "" {
		parsed, err := parseSteamID(rawSteamID)
		if err != nil {
			httpx.RespondWithError(responseWriter, http.StatusBadRequest, err.Error())
			return
		}
		enteredID = parsed
	}

	ctx := request.Context()
	var (
		steamUser dbgen.SteamUser
		steamID   int64
		err       error
	)
	if vanityUrl != "" {
		// The vanity path is unchanged except that the import always wins
		// over a stored custom label: the upsert clears the flag.
		// customUsername is ignored here.
		steamID, steamUser, err = handler.importSteamUserByVanity(ctx, vanityUrl, enteredID)
	} else {
		steamID, steamUser, err = handler.importSteamUserByID(ctx, enteredID, strings.TrimSpace(body.CustomUsername))
	}
	if err != nil {
		handler.respondImportError(responseWriter, request, err)
		return
	}

	requestImport := handler.importer.RequestImport
	if body.Force {
		requestImport = handler.importer.RequestForcedImport
	}
	job, created, err := requestImport(ctx, steamID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusAccepted
		responseWriter.Header().Set("Location", importJobLocation(job.ID))
	}
	httpx.RespondWithJSON(responseWriter, status, insertSteamUserResponse{
		SteamUser: newSteamUserView(steamUser),
		ImportJob: newImportJobView(job),
	})
}

// importSteamUserByVanity resolves the custom URL name, refuses a stale
// preview, and stores the row under the vanity with the custom-name flag
// cleared.
func (handler *Handler) importSteamUserByVanity(ctx context.Context, vanityUrl string, enteredID int64) (int64, dbgen.SteamUser, error) {
	steamID, player, err := handler.resolveSteamPlayer(ctx, vanityUrl)
	if err != nil {
		return 0, dbgen.SteamUser{}, err
	}

	// The preview may be stale: the name could have been reassigned between
	// the lookup and the click. Refuse to import the wrong account.
	if enteredID != 0 && enteredID != steamID {
		return 0, dbgen.SteamUser{}, errSteamPreviewMismatch
	}

	steamUser, err := handler.storeSteamUser(ctx, steamID, vanityUrl, false, player)
	if err != nil {
		return 0, dbgen.SteamUser{}, err
	}
	return steamID, steamUser, nil
}

// importSteamUserByID skips vanity resolution entirely: the entered ID is
// validated with a read-only profile fetch, the effective name and flag are
// resolved from the optional label, and the row is stored under them.
func (handler *Handler) importSteamUserByID(ctx context.Context, steamID int64, label string) (int64, dbgen.SteamUser, error) {
	player, err := handler.fetchSteamPlayerByID(ctx, steamID)
	if err != nil {
		return 0, dbgen.SteamUser{}, err
	}

	name, custom, err := handler.resolveImportName(ctx, steamID, label)
	if err != nil {
		return 0, dbgen.SteamUser{}, err
	}
	steamUser, err := handler.storeSteamUser(ctx, steamID, name, custom, player)
	if errors.Is(err, errSteamUniqueViolation) {
		// A concurrent import claimed the label between the pre-check and
		// the upsert; the partial unique index rejected it.
		return 0, dbgen.SteamUser{}, errSteamUsernameTaken
	}
	if err != nil {
		return 0, dbgen.SteamUser{}, err
	}
	return steamID, steamUser, nil
}

// resolveImportName decides the stored name and flag for an ID import: a
// provided label wins (after the uniqueness pre-check), a blank label keeps
// the row's current values, and a brand-new user falls back to the canonical
// Steam ID digits as a custom name.
func (handler *Handler) resolveImportName(ctx context.Context, steamID int64, label string) (string, bool, error) {
	if label != "" {
		if utf8.RuneCountInString(label) > maxSteamUserNameLength {
			return "", false, errSteamNameTooLong
		}
		existing, err := handler.apiStore.queries.GetSteamUserByCustomName(ctx, label)
		if err == nil && existing.SteamID != steamID {
			return "", false, errSteamUsernameTaken
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", false, fmt.Errorf("check custom username: %w", err)
		}
		return label, true, nil
	}

	existing, err := handler.apiStore.queries.GetSteamUser(ctx, steamID)
	if err == nil {
		return existing.Name, existing.CustomUsername, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return formatSteamID(steamID), true, nil
	}
	return "", false, fmt.Errorf("get steam user: %w", err)
}

// storeSteamUser upserts the profile fields, name, and custom-name flag.
// Nothing is stored before the callers have validated identity inputs.
func (handler *Handler) storeSteamUser(ctx context.Context, steamID int64, name string, custom bool, player steamapi.PlayerSummary) (dbgen.SteamUser, error) {
	steamUser, err := handler.apiStore.queries.UpsertSteamUser(ctx, dbgen.UpsertSteamUserParams{
		SteamID:        steamID,
		Name:           name,
		Personaname:    nullString(player.PersonaName),
		Profileurl:     nullString(player.ProfileURL),
		Avatar:         nullString(player.Avatar),
		AvatarMedium:   nullString(player.AvatarMedium),
		AvatarFull:     nullString(player.AvatarFull),
		CustomUsername: custom,
	})
	if isUniqueViolation(err) {
		return dbgen.SteamUser{}, errSteamUniqueViolation
	}
	if err != nil {
		return dbgen.SteamUser{}, fmt.Errorf("upsert steam user: %w", err)
	}
	return steamUser, nil
}

var errSteamUniqueViolation = errors.New("steam user unique violation")

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// errSteamPreviewMismatch wraps the vanity-path identity rejection.
var errSteamPreviewMismatch = errors.New("steam preview identity mismatch")

// respondImportError maps the import preconditions to their HTTP status
// codes; everything unexpected is a 500.
func (handler *Handler) respondImportError(responseWriter http.ResponseWriter, request *http.Request, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, errSteamUserNotFound):
		httpx.RespondWithError(responseWriter, http.StatusNotFound, "steam user not found")
	case errors.Is(err, errSteamPreviewMismatch):
		httpx.RespondWithError(responseWriter, http.StatusConflict,
			"The Steam name now resolves to a different user. Look up the user again.")
	case errors.Is(err, errSteamUsernameTaken):
		httpx.RespondWithError(responseWriter, http.StatusConflict, steamUsernameTakenMessage)
	case errors.Is(err, errSteamNameTooLong):
		httpx.RespondWithError(responseWriter, http.StatusBadRequest,
			fmt.Sprintf("customUsername must be at most %d characters", maxSteamUserNameLength))
	default:
		handler.internalError(responseWriter, request, err)
	}
}
