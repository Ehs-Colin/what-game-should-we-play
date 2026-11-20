package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/auth"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/logging"
)

type AuthService interface {
	Login(ctx context.Context, username, password string) (auth.LoginResult, error)
	Logout(ctx context.Context, token string) error
	ResolveSession(ctx context.Context, token string) (auth.Principal, error)
	CreateRealAccount(ctx context.Context, current auth.Principal, defaultPassword, username, password string) (auth.LoginResult, error)
}

type Handler struct {
	apiStore     *Store
	importer     ImportRequester
	logger       *logging.Logger
	auth         AuthService
	cookieSecure bool
	loginLimiter *LoginThrottle
	clientIP     func(*http.Request) string
}

func NewHandler(
	apiStore *Store,
	importer ImportRequester,
	logger *logging.Logger,
	authService AuthService,
	cookieSecure bool,
	loginLimiter *LoginThrottle,
	clientIP func(*http.Request) string,
) *Handler {
	if clientIP == nil {
		clientIP = httpx.ClientIPKey
	}
	return &Handler{
		apiStore:     apiStore,
		importer:     importer,
		logger:       logger,
		auth:         authService,
		cookieSecure: cookieSecure,
		loginLimiter: loginLimiter,
		clientIP:     clientIP,
	}
}

func parseSteamIDs(rawIDs []string) ([]int64, error) {
	seen := make(map[int64]struct{}, len(rawIDs))
	steamIDs := make([]int64, 0, len(rawIDs))
	for _, rawSteamID := range rawIDs {
		value := strings.TrimSpace(rawSteamID)
		if value == "" {
			return nil, errors.New("steamIds must not contain blank values")
		}
		steamID, err := strconv.ParseInt(value, 10, 64)
		if err != nil || steamID <= 0 {
			return nil, errors.New("steamIds must contain positive SteamID64 values")
		}
		if _, duplicate := seen[steamID]; duplicate {
			continue
		}
		seen[steamID] = struct{}{}
		steamIDs = append(steamIDs, steamID)
	}
	if len(steamIDs) == 0 {
		return nil, errors.New("steamIds is required")
	}
	if len(steamIDs) > maxCommonGameUsers {
		return nil, fmt.Errorf("at most %d unique steamIds are allowed", maxCommonGameUsers)
	}
	return steamIDs, nil
}

func (handler *Handler) internalError(responseWriter http.ResponseWriter, request *http.Request, err error) {
	if handler.logger != nil {
		_ = handler.logger.Event("unhandled_error", map[string]any{
			"method":  request.Method,
			"path":    request.URL.Path,
			"message": err.Error(),
		})
	}
	httpx.RespondWithError(responseWriter, http.StatusInternalServerError, err.Error())
}
