package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/logging"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	SessionCookieName      = "wgs_session"
	SessionIdleTimeout     = 30 * time.Minute
	SessionAbsoluteTimeout = 24 * time.Hour
	sessionTouchInterval   = time.Minute
	minPasswordLength      = 12
	maxPasswordLength      = 128
	minUsernameLength      = 3
	maxUsernameLength      = 64

	// defaultUserAdvisoryLockKey serializes default-account bootstrap across
	// server instances. The value is arbitrary but must stay stable.
	defaultUserAdvisoryLockKey int64 = 0x7767735f61757468
)

var (
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrNoSession               = errors.New("no session")
	ErrAccountCreationRequired = errors.New("account creation required")
	ErrRealAccountExists       = errors.New("real account already exists")
	ErrUsernameTaken           = errors.New("username is already taken")
	ErrInvalidUsername         = errors.New("invalid username")
	ErrInvalidPassword         = errors.New("invalid password")
)

// dummyPasswordHash is a valid Argon2id hash used to equalize login timing
// when no matching account exists.
const dummyPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$Ms3257xpwUlyQEUk5Lgdrg$164dNAjDEzmkvqYJ7pgPiXCyjizl4kyGOz3fUzTXG2Q"

type Principal struct {
	UserID            int64
	Username          string
	IsDefault         bool
	MustCreateAccount bool
	CSRFToken         string
}

type LoginResult struct {
	Principal Principal
	Token     string
	ExpiresAt time.Time
}

type Manager struct {
	db      *sql.DB
	queries *dbgen.Queries
	logger  *logging.Logger
	now     func() time.Time
}

func NewManager(database *sql.DB, logger *logging.Logger) *Manager {
	return &Manager{
		db:      database,
		queries: dbgen.New(database),
		logger:  logger,
		now:     time.Now,
	}
}

// EnsureDefaultUser creates the bootstrap account when app_users is empty. If
// any account already exists it does nothing, so the environment values are
// only read on the very first startup.
func (manager *Manager) EnsureDefaultUser(ctx context.Context, username, password string) error {
	transaction, err := manager.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin default user transaction: %w", err)
	}
	defer transaction.Rollback()

	if _, err := transaction.ExecContext(
		ctx,
		"SELECT pg_advisory_xact_lock($1)",
		defaultUserAdvisoryLockKey,
	); err != nil {
		return fmt.Errorf("lock default user bootstrap: %w", err)
	}

	queries := manager.queries.WithTx(transaction)
	userCount, err := queries.CountAppUsers(ctx)
	if err != nil {
		return fmt.Errorf("count app users: %w", err)
	}
	if userCount > 0 {
		if err := transaction.Commit(); err != nil {
			return fmt.Errorf("commit default user transaction: %w", err)
		}
		return nil
	}

	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return errors.New("no accounts exist; set DEFAULT_ADMIN_USERNAME and DEFAULT_ADMIN_PASSWORD")
	}
	if err := validateUsername(username); err != nil {
		return fmt.Errorf("DEFAULT_ADMIN_USERNAME: %w", err)
	}
	if err := validatePassword(password); err != nil {
		return fmt.Errorf("DEFAULT_ADMIN_PASSWORD: %w", err)
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash default password: %w", err)
	}

	user, err := queries.CreateAppUser(ctx, dbgen.CreateAppUserParams{
		Username:     username,
		PasswordHash: passwordHash,
		IsDefault:    true,
		Now:          manager.now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("create default user: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit default user transaction: %w", err)
	}

	manager.logEvent("auth_default_user_created", map[string]any{
		"userId":   user.ID,
		"username": user.Username,
	})
	return nil
}

// Login verifies credentials and creates a session. Every credential failure
// returns ErrInvalidCredentials, including disabled accounts and a default
// account that has already been retired.
func (manager *Manager) Login(ctx context.Context, username, password string) (LoginResult, error) {
	username = strings.TrimSpace(username)

	user, err := manager.queries.GetAppUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Spend the same work as a real verification to limit username
			// enumeration through response timing.
			_, _ = VerifyPassword(dummyPasswordHash, password)
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, fmt.Errorf("load app user: %w", err)
	}

	if user.DisabledAt.Valid {
		return LoginResult{}, ErrInvalidCredentials
	}
	if user.IsDefault {
		realUserCount, err := manager.queries.CountRealAppUsers(ctx)
		if err != nil {
			return LoginResult{}, fmt.Errorf("count real app users: %w", err)
		}
		if realUserCount > 0 {
			return LoginResult{}, ErrInvalidCredentials
		}
	}

	valid, err := VerifyPassword(user.PasswordHash, password)
	if err != nil || !valid {
		return LoginResult{}, ErrInvalidCredentials
	}

	result, err := manager.createSession(ctx, manager.queries, user)
	if err != nil {
		return LoginResult{}, err
	}
	if err := manager.queries.TouchAppUserLogin(ctx, dbgen.TouchAppUserLoginParams{
		Now: manager.now().UTC(),
		ID:  user.ID,
	}); err != nil {
		return LoginResult{}, fmt.Errorf("record login: %w", err)
	}

	if err := manager.PruneExpiredSessions(ctx); err != nil {
		manager.logEvent("auth_session_prune_failed", map[string]any{"error": err.Error()})
	}
	return result, nil
}

// ResolveSession validates a raw session token and returns its principal.
// Expired, disabled, and retired-default sessions are deleted.
func (manager *Manager) ResolveSession(ctx context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, ErrNoSession
	}

	session, err := manager.queries.GetAppSession(ctx, HashSessionToken(token))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Principal{}, ErrNoSession
		}
		return Principal{}, fmt.Errorf("load app session: %w", err)
	}

	now := manager.now().UTC()
	discard := func() {
		if err := manager.queries.DeleteAppSession(ctx, session.TokenHash); err != nil {
			manager.logEvent("auth_session_delete_failed", map[string]any{"error": err.Error()})
		}
	}

	if session.DisabledAt.Valid {
		discard()
		return Principal{}, ErrNoSession
	}
	if session.IsDefault {
		realUserCount, err := manager.queries.CountRealAppUsers(ctx)
		if err != nil {
			return Principal{}, fmt.Errorf("count real app users: %w", err)
		}
		if realUserCount > 0 {
			discard()
			return Principal{}, ErrNoSession
		}
	}
	if !now.Before(session.ExpiresAt) {
		discard()
		return Principal{}, ErrNoSession
	}
	if !now.Before(session.LastSeenAt.Add(SessionIdleTimeout)) {
		discard()
		return Principal{}, ErrNoSession
	}

	if now.Sub(session.LastSeenAt) >= sessionTouchInterval {
		if err := manager.queries.TouchAppSession(ctx, dbgen.TouchAppSessionParams{
			Now:       now,
			TokenHash: session.TokenHash,
		}); err != nil {
			manager.logEvent("auth_session_touch_failed", map[string]any{"error": err.Error()})
		}
	}

	return Principal{
		UserID:            session.UserID,
		Username:          session.Username,
		IsDefault:         session.IsDefault,
		MustCreateAccount: session.IsDefault,
		CSRFToken:         session.CsrfToken,
	}, nil
}

// CreateRealAccount promotes the signed-in default account by creating the one
// real account, disabling the default account, and returning a session for the
// new account. The whole change is one transaction.
func (manager *Manager) CreateRealAccount(
	ctx context.Context,
	current Principal,
	defaultPassword string,
	username string,
	password string,
) (LoginResult, error) {
	if !current.IsDefault {
		return LoginResult{}, ErrAccountCreationRequired
	}

	transaction, err := manager.db.BeginTx(ctx, nil)
	if err != nil {
		return LoginResult{}, fmt.Errorf("begin account creation transaction: %w", err)
	}
	defer transaction.Rollback()

	queries := manager.queries.WithTx(transaction)
	defaultUser, err := queries.GetDefaultAppUserForUpdate(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginResult{}, ErrAccountCreationRequired
		}
		return LoginResult{}, fmt.Errorf("load default app user: %w", err)
	}
	if defaultUser.ID != current.UserID ||
		!defaultUser.IsDefault ||
		defaultUser.DisabledAt.Valid {
		return LoginResult{}, ErrAccountCreationRequired
	}

	valid, err := VerifyPassword(defaultUser.PasswordHash, defaultPassword)
	if err != nil || !valid {
		return LoginResult{}, ErrInvalidCredentials
	}

	realUserCount, err := queries.CountRealAppUsers(ctx)
	if err != nil {
		return LoginResult{}, fmt.Errorf("count real app users: %w", err)
	}
	if realUserCount > 0 {
		return LoginResult{}, ErrRealAccountExists
	}

	username = strings.TrimSpace(username)
	if err := validateUsername(username); err != nil {
		return LoginResult{}, err
	}
	if err := validatePassword(password); err != nil {
		return LoginResult{}, err
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return LoginResult{}, fmt.Errorf("hash password: %w", err)
	}

	now := manager.now().UTC()
	user, err := queries.CreateAppUser(ctx, dbgen.CreateAppUserParams{
		Username:     username,
		PasswordHash: passwordHash,
		IsDefault:    false,
		Now:          now,
	})
	if err != nil {
		return LoginResult{}, mapCreateAppUserError(err)
	}

	if err := queries.DisableAppUser(ctx, dbgen.DisableAppUserParams{
		Now: now,
		ID:  defaultUser.ID,
	}); err != nil {
		return LoginResult{}, fmt.Errorf("disable default app user: %w", err)
	}
	if err := queries.DeleteAppSessionsForUser(ctx, defaultUser.ID); err != nil {
		return LoginResult{}, fmt.Errorf("delete default app user sessions: %w", err)
	}

	result, err := manager.createSession(ctx, queries, user)
	if err != nil {
		return LoginResult{}, err
	}
	if err := transaction.Commit(); err != nil {
		return LoginResult{}, fmt.Errorf("commit account creation transaction: %w", err)
	}

	manager.logEvent("auth_real_account_created", map[string]any{
		"userId":   user.ID,
		"username": user.Username,
	})
	return result, nil
}

// Logout deletes the session for token. It is idempotent, including when the
// token is empty or the row is already gone.
func (manager *Manager) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := manager.queries.DeleteAppSession(ctx, HashSessionToken(token)); err != nil {
		return fmt.Errorf("delete app session: %w", err)
	}
	return nil
}

// PruneExpiredSessions deletes sessions past their absolute or idle expiry.
func (manager *Manager) PruneExpiredSessions(ctx context.Context) error {
	now := manager.now().UTC()
	if err := manager.queries.DeleteExpiredAppSessions(ctx, dbgen.DeleteExpiredAppSessionsParams{
		Now:        now,
		IdleCutoff: now.Add(-SessionIdleTimeout),
	}); err != nil {
		return fmt.Errorf("delete expired app sessions: %w", err)
	}
	return nil
}

func (manager *Manager) createSession(
	ctx context.Context,
	queries *dbgen.Queries,
	user dbgen.AppUser,
) (LoginResult, error) {
	token, tokenHash, err := NewSessionToken()
	if err != nil {
		return LoginResult{}, err
	}
	csrfToken, err := NewCSRFToken()
	if err != nil {
		return LoginResult{}, err
	}

	now := manager.now().UTC()
	expiresAt := now.Add(SessionAbsoluteTimeout)
	if err := queries.CreateAppSession(ctx, dbgen.CreateAppSessionParams{
		TokenHash:  tokenHash,
		UserID:     user.ID,
		CsrfToken:  csrfToken,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  expiresAt,
	}); err != nil {
		return LoginResult{}, fmt.Errorf("create app session: %w", err)
	}

	return LoginResult{
		Principal: Principal{
			UserID:            user.ID,
			Username:          user.Username,
			IsDefault:         user.IsDefault,
			MustCreateAccount: user.IsDefault,
			CSRFToken:         csrfToken,
		},
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (manager *Manager) logEvent(eventName string, fields map[string]any) {
	if manager.logger == nil {
		return
	}
	_ = manager.logger.Event(eventName, fields)
}

func validateUsername(username string) error {
	if len(username) < minUsernameLength || len(username) > maxUsernameLength {
		return fmt.Errorf(
			"%w: username must be between %d and %d characters",
			ErrInvalidUsername,
			minUsernameLength,
			maxUsernameLength,
		)
	}
	for index, character := range username {
		alphanumeric := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9'
		if index == 0 && !alphanumeric {
			return fmt.Errorf("%w: username must start with a letter or number", ErrInvalidUsername)
		}
		if !alphanumeric && character != '.' && character != '_' && character != '-' {
			return fmt.Errorf("%w: username may only contain letters, numbers, '.', '_', and '-'", ErrInvalidUsername)
		}
	}
	return nil
}

func mapCreateAppUserError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		switch postgresError.ConstraintName {
		case "app_users_single_real":
			return ErrRealAccountExists
		case "app_users_username_lower":
			return ErrUsernameTaken
		}
	}
	return fmt.Errorf("create app user: %w", err)
}
