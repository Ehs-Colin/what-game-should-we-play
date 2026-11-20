package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbtest"
)

const (
	managerTestDefaultUsername = "admin"
	managerTestDefaultPassword = "default-password-123"
	managerTestRealUsername    = "player"
	managerTestRealPassword    = "real-password-456"
)

type managerTestClock struct {
	now time.Time
}

func (clock *managerTestClock) Now() time.Time {
	return clock.now
}

func (clock *managerTestClock) Advance(duration time.Duration) {
	clock.now = clock.now.Add(duration)
}

type managerEnv struct {
	db      *sql.DB
	queries *dbgen.Queries
	manager *Manager
	clock   *managerTestClock
}

func newManagerEnv(t *testing.T) *managerEnv {
	t.Helper()
	db := dbtest.Open(t)
	clock := &managerTestClock{
		now: time.Date(2026, time.March, 4, 5, 6, 7, 0, time.UTC),
	}
	manager := NewManager(db, nil)
	manager.now = clock.Now
	return &managerEnv{
		db:      db,
		queries: dbgen.New(db),
		manager: manager,
		clock:   clock,
	}
}

func (env *managerEnv) ensureDefault(t *testing.T) {
	t.Helper()
	if err := env.manager.EnsureDefaultUser(
		context.Background(),
		managerTestDefaultUsername,
		managerTestDefaultPassword,
	); err != nil {
		t.Fatalf("ensure default user: %v", err)
	}
}

func (env *managerEnv) login(t *testing.T, username, password string) LoginResult {
	t.Helper()
	result, err := env.manager.Login(context.Background(), username, password)
	if err != nil {
		t.Fatalf("login %q: %v", username, err)
	}
	return result
}

func (env *managerEnv) createUser(t *testing.T, username, password string, isDefault bool) dbgen.AppUser {
	t.Helper()
	passwordHash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user, err := env.queries.CreateAppUser(context.Background(), dbgen.CreateAppUserParams{
		Username:     username,
		PasswordHash: passwordHash,
		IsDefault:    isDefault,
		Now:          env.clock.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create user %q: %v", username, err)
	}
	return user
}

func (env *managerEnv) disableUser(t *testing.T, userID int64) {
	t.Helper()
	if err := env.queries.DisableAppUser(context.Background(), dbgen.DisableAppUserParams{
		Now: env.clock.Now().UTC(),
		ID:  userID,
	}); err != nil {
		t.Fatalf("disable user %d: %v", userID, err)
	}
}

func (env *managerEnv) countUsers(t *testing.T) int {
	t.Helper()
	count, err := env.queries.CountAppUsers(context.Background())
	if err != nil {
		t.Fatalf("count users: %v", err)
	}
	return int(count)
}

func (env *managerEnv) countRealUsers(t *testing.T) int {
	t.Helper()
	count, err := env.queries.CountRealAppUsers(context.Background())
	if err != nil {
		t.Fatalf("count real users: %v", err)
	}
	return int(count)
}

func (env *managerEnv) countSessions(t *testing.T) int {
	t.Helper()
	var count int
	if err := env.db.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM app_sessions",
	).Scan(&count); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	return count
}

func (env *managerEnv) sessionExists(t *testing.T, token string) bool {
	t.Helper()
	_, err := env.queries.GetAppSession(context.Background(), HashSessionToken(token))
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	return true
}

func TestEnsureDefaultUserCreatesDefaultAccount(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)

	if got := env.countUsers(t); got != 1 {
		t.Fatalf("%d users, want 1", got)
	}
	user, err := env.queries.GetAppUserByUsername(context.Background(), managerTestDefaultUsername)
	if err != nil {
		t.Fatalf("load default user: %v", err)
	}
	if !user.IsDefault {
		t.Fatal("created user is not marked as the default account")
	}
	if user.DisabledAt.Valid {
		t.Fatal("created default account is disabled")
	}
	if !strings.HasPrefix(user.PasswordHash, "$argon2id$") {
		t.Fatalf("password hash = %q, want an argon2id hash", user.PasswordHash)
	}
	if user.LastLoginAt.Valid {
		t.Fatal("bootstrap recorded a login")
	}
}

func TestEnsureDefaultUserIsIdempotent(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)

	original, err := env.queries.GetAppUserByUsername(context.Background(), managerTestDefaultUsername)
	if err != nil {
		t.Fatalf("load default user: %v", err)
	}

	if err := env.manager.EnsureDefaultUser(
		context.Background(),
		"someone-else",
		"different-password-123",
	); err != nil {
		t.Fatalf("ensure default user again: %v", err)
	}

	if got := env.countUsers(t); got != 1 {
		t.Fatalf("%d users after second bootstrap, want 1", got)
	}
	current, err := env.queries.GetAppUserByUsername(context.Background(), managerTestDefaultUsername)
	if err != nil {
		t.Fatalf("reload default user: %v", err)
	}
	if current.ID != original.ID || current.Username != original.Username {
		t.Fatalf("default user changed from %+v to %+v", original, current)
	}
	if current.PasswordHash != original.PasswordHash {
		t.Fatal("second bootstrap overwrote the default password")
	}

	env.login(t, managerTestDefaultUsername, managerTestDefaultPassword)
	if _, err := env.manager.Login(context.Background(), managerTestDefaultUsername, "different-password-123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("login with second bootstrap password: error = %v, want ErrInvalidCredentials", err)
	}
}

func TestEnsureDefaultUserDoesNothingWhenRealAccountExists(t *testing.T) {
	env := newManagerEnv(t)
	env.createUser(t, managerTestRealUsername, managerTestRealPassword, false)

	if err := env.manager.EnsureDefaultUser(
		context.Background(),
		managerTestDefaultUsername,
		managerTestDefaultPassword,
	); err != nil {
		t.Fatalf("ensure default user: %v", err)
	}

	if got := env.countUsers(t); got != 1 {
		t.Fatalf("%d users, want only the existing real account", got)
	}
	if _, err := env.queries.GetAppUserByUsername(context.Background(), managerTestDefaultUsername); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("default account was created despite a real account; error = %v", err)
	}
}

func TestEnsureDefaultUserRequiresCredentials(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{"both missing", "", ""},
		{"missing username", "", managerTestDefaultPassword},
		{"missing password", managerTestDefaultUsername, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := newManagerEnv(t)
			err := env.manager.EnsureDefaultUser(context.Background(), test.username, test.password)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), "no accounts exist; set DEFAULT_ADMIN_USERNAME and DEFAULT_ADMIN_PASSWORD") {
				t.Fatalf("error = %q, want the missing-credentials message", err)
			}
			if got := env.countUsers(t); got != 0 {
				t.Fatalf("%d users created, want 0", got)
			}
		})
	}
}

func TestLoginDefaultAccount(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)

	result := env.login(t, managerTestDefaultUsername, managerTestDefaultPassword)
	if result.Token == "" {
		t.Fatal("login returned an empty session token")
	}
	if result.Principal.CSRFToken == "" {
		t.Fatal("login returned an empty CSRF token")
	}
	if result.Principal.UserID == 0 {
		t.Fatal("login returned a zero user ID")
	}
	if result.Principal.Username != managerTestDefaultUsername {
		t.Fatalf("username = %q, want %q", result.Principal.Username, managerTestDefaultUsername)
	}
	if !result.Principal.IsDefault || !result.Principal.MustCreateAccount {
		t.Fatalf("principal = %+v, want a default account that must create a real account", result.Principal)
	}
	if want := env.clock.Now().Add(SessionAbsoluteTimeout); !result.ExpiresAt.Equal(want) {
		t.Fatalf("expires at = %v, want %v", result.ExpiresAt, want)
	}
	if got := env.countSessions(t); got != 1 {
		t.Fatalf("%d sessions, want 1", got)
	}

	principal, err := env.manager.ResolveSession(context.Background(), result.Token)
	if err != nil {
		t.Fatalf("resolve session: %v", err)
	}
	if principal != result.Principal {
		t.Fatalf("resolved principal = %+v, want %+v", principal, result.Principal)
	}

	user, err := env.queries.GetAppUserByUsername(context.Background(), managerTestDefaultUsername)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	if !user.LastLoginAt.Valid || !user.LastLoginAt.Time.Equal(env.clock.Now()) {
		t.Fatalf("last login = %v, want %v", user.LastLoginAt, env.clock.Now())
	}
}

func TestLoginIsCaseInsensitive(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)

	result := env.login(t, "ADMIN", managerTestDefaultPassword)
	if result.Principal.Username != managerTestDefaultUsername {
		t.Fatalf("username = %q, want %q", result.Principal.Username, managerTestDefaultUsername)
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		setup    func(t *testing.T, env *managerEnv)
	}{
		{
			name:     "wrong password",
			username: managerTestDefaultUsername,
			password: "wrong-password-123",
		},
		{
			name:     "unknown username",
			username: "nobody",
			password: managerTestDefaultPassword,
		},
		{
			name:     "disabled account",
			username: managerTestDefaultUsername,
			password: managerTestDefaultPassword,
			setup: func(t *testing.T, env *managerEnv) {
				user, err := env.queries.GetAppUserByUsername(context.Background(), managerTestDefaultUsername)
				if err != nil {
					t.Fatalf("load default user: %v", err)
				}
				env.disableUser(t, user.ID)
			},
		},
		{
			name:     "retired default account",
			username: managerTestDefaultUsername,
			password: managerTestDefaultPassword,
			setup: func(t *testing.T, env *managerEnv) {
				env.createUser(t, managerTestRealUsername, managerTestRealPassword, false)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := newManagerEnv(t)
			env.ensureDefault(t)
			if test.setup != nil {
				test.setup(t, env)
			}

			result, err := env.manager.Login(context.Background(), test.username, test.password)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("error = %v, want ErrInvalidCredentials", err)
			}
			if result != (LoginResult{}) {
				t.Fatalf("result = %+v, want zero value", result)
			}
			if got := env.countSessions(t); got != 0 {
				t.Fatalf("%d sessions, want 0", got)
			}
		})
	}
}

func TestResolveSessionRejectsExpiredSessions(t *testing.T) {
	tests := []struct {
		name    string
		advance time.Duration
	}{
		{"absolute expiry", SessionAbsoluteTimeout},
		{"idle expiry", SessionIdleTimeout},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := newManagerEnv(t)
			env.ensureDefault(t)
			result := env.login(t, managerTestDefaultUsername, managerTestDefaultPassword)

			env.clock.Advance(test.advance)

			principal, err := env.manager.ResolveSession(context.Background(), result.Token)
			if !errors.Is(err, ErrNoSession) {
				t.Fatalf("error = %v, want ErrNoSession", err)
			}
			if principal != (Principal{}) {
				t.Fatalf("principal = %+v, want zero value", principal)
			}
			if env.sessionExists(t, result.Token) {
				t.Fatal("expired session was not deleted")
			}
		})
	}
}

func TestResolveSessionRejectsDisabledUser(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)
	result := env.login(t, managerTestDefaultUsername, managerTestDefaultPassword)

	env.disableUser(t, result.Principal.UserID)

	if _, err := env.manager.ResolveSession(context.Background(), result.Token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("error = %v, want ErrNoSession", err)
	}
	if env.sessionExists(t, result.Token) {
		t.Fatal("session for disabled user was not deleted")
	}
}

func TestResolveSessionRejectsRetiredDefaultUser(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)
	result := env.login(t, managerTestDefaultUsername, managerTestDefaultPassword)

	env.createUser(t, managerTestRealUsername, managerTestRealPassword, false)

	if _, err := env.manager.ResolveSession(context.Background(), result.Token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("error = %v, want ErrNoSession", err)
	}
	if env.sessionExists(t, result.Token) {
		t.Fatal("session for retired default user was not deleted")
	}
}

func TestLogoutDeletesSessionAndIsIdempotent(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)
	result := env.login(t, managerTestDefaultUsername, managerTestDefaultPassword)

	if err := env.manager.Logout(context.Background(), result.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if env.sessionExists(t, result.Token) {
		t.Fatal("session still exists after logout")
	}
	if got := env.countSessions(t); got != 0 {
		t.Fatalf("%d sessions after logout, want 0", got)
	}

	if err := env.manager.Logout(context.Background(), result.Token); err != nil {
		t.Fatalf("logout again: %v", err)
	}
	if err := env.manager.Logout(context.Background(), ""); err != nil {
		t.Fatalf("logout with empty token: %v", err)
	}
}

func TestCreateRealAccount(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)
	defaultLogin := env.login(t, managerTestDefaultUsername, managerTestDefaultPassword)

	result, err := env.manager.CreateRealAccount(
		context.Background(),
		defaultLogin.Principal,
		managerTestDefaultPassword,
		managerTestRealUsername,
		managerTestRealPassword,
	)
	if err != nil {
		t.Fatalf("create real account: %v", err)
	}

	if result.Principal.Username != managerTestRealUsername {
		t.Fatalf("username = %q, want %q", result.Principal.Username, managerTestRealUsername)
	}
	if result.Principal.IsDefault || result.Principal.MustCreateAccount {
		t.Fatalf("principal = %+v, want a real account that is ready to use", result.Principal)
	}
	if result.Token == "" || result.Principal.CSRFToken == "" {
		t.Fatal("account creation returned an empty session or CSRF token")
	}
	if got := env.countRealUsers(t); got != 1 {
		t.Fatalf("%d real users, want 1", got)
	}
	if got := env.countSessions(t); got != 1 {
		t.Fatalf("%d sessions, want only the new session", got)
	}

	defaultUser, err := env.queries.GetAppUserByUsername(context.Background(), managerTestDefaultUsername)
	if err != nil {
		t.Fatalf("load default user: %v", err)
	}
	if !defaultUser.DisabledAt.Valid {
		t.Fatal("default account was not disabled")
	}
	if env.sessionExists(t, defaultLogin.Token) {
		t.Fatal("default account session was not deleted")
	}

	principal, err := env.manager.ResolveSession(context.Background(), result.Token)
	if err != nil {
		t.Fatalf("resolve real session: %v", err)
	}
	if principal != result.Principal {
		t.Fatalf("resolved principal = %+v, want %+v", principal, result.Principal)
	}

	if _, err := env.manager.Login(context.Background(), managerTestDefaultUsername, managerTestDefaultPassword); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("default login after promotion: error = %v, want ErrInvalidCredentials", err)
	}
}

func TestCreateRealAccountRejectsWrongDefaultPassword(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)
	defaultLogin := env.login(t, managerTestDefaultUsername, managerTestDefaultPassword)

	_, err := env.manager.CreateRealAccount(
		context.Background(),
		defaultLogin.Principal,
		"wrong-default-password",
		managerTestRealUsername,
		managerTestRealPassword,
	)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("error = %v, want ErrInvalidCredentials", err)
	}

	if got := env.countRealUsers(t); got != 0 {
		t.Fatalf("%d real users, want 0", got)
	}
	defaultUser, err := env.queries.GetAppUserByUsername(context.Background(), managerTestDefaultUsername)
	if err != nil {
		t.Fatalf("load default user: %v", err)
	}
	if defaultUser.DisabledAt.Valid {
		t.Fatal("default account was disabled after a failed attempt")
	}
	if !env.sessionExists(t, defaultLogin.Token) {
		t.Fatal("default session was deleted after a failed attempt")
	}
	if _, err := env.manager.ResolveSession(context.Background(), defaultLogin.Token); err != nil {
		t.Fatalf("default session no longer resolves: %v", err)
	}
}

func TestCreateRealAccountRejectsNonDefaultCaller(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)
	defaultLogin := env.login(t, managerTestDefaultUsername, managerTestDefaultPassword)

	realLogin, err := env.manager.CreateRealAccount(
		context.Background(),
		defaultLogin.Principal,
		managerTestDefaultPassword,
		managerTestRealUsername,
		managerTestRealPassword,
	)
	if err != nil {
		t.Fatalf("create real account: %v", err)
	}

	if _, err := env.manager.CreateRealAccount(
		context.Background(),
		realLogin.Principal,
		managerTestRealPassword,
		"another-player",
		"another-password-789",
	); !errors.Is(err, ErrAccountCreationRequired) {
		t.Fatalf("error = %v, want ErrAccountCreationRequired", err)
	}
	if got := env.countUsers(t); got != 2 {
		t.Fatalf("%d users, want 2", got)
	}
}

func TestCreateRealAccountRejectsSecondRealAccount(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)
	defaultLogin := env.login(t, managerTestDefaultUsername, managerTestDefaultPassword)

	env.createUser(t, managerTestRealUsername, managerTestRealPassword, false)

	if _, err := env.manager.CreateRealAccount(
		context.Background(),
		defaultLogin.Principal,
		managerTestDefaultPassword,
		"another-player",
		"another-password-789",
	); !errors.Is(err, ErrRealAccountExists) {
		t.Fatalf("error = %v, want ErrRealAccountExists", err)
	}
	if got := env.countRealUsers(t); got != 1 {
		t.Fatalf("%d real users, want 1", got)
	}
	if got := env.countUsers(t); got != 2 {
		t.Fatalf("%d users, want 2", got)
	}
	defaultUser, err := env.queries.GetAppUserByUsername(context.Background(), managerTestDefaultUsername)
	if err != nil {
		t.Fatalf("load default user: %v", err)
	}
	if defaultUser.DisabledAt.Valid {
		t.Fatal("default account was disabled after a failed attempt")
	}
}

func TestCreateRealAccountRejectsDuplicateUsername(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)
	defaultLogin := env.login(t, managerTestDefaultUsername, managerTestDefaultPassword)

	_, err := env.manager.CreateRealAccount(
		context.Background(),
		defaultLogin.Principal,
		managerTestDefaultPassword,
		strings.ToUpper(managerTestDefaultUsername),
		managerTestRealPassword,
	)
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("error = %v, want ErrUsernameTaken", err)
	}
}

func TestCreateRealAccountRollsBackInsertFailure(t *testing.T) {
	env := newManagerEnv(t)
	env.ensureDefault(t)
	defaultLogin := env.login(t, managerTestDefaultUsername, managerTestDefaultPassword)

	if _, err := env.manager.CreateRealAccount(
		context.Background(),
		defaultLogin.Principal,
		managerTestDefaultPassword,
		strings.ToUpper(managerTestDefaultUsername),
		managerTestRealPassword,
	); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("error = %v, want ErrUsernameTaken", err)
	}

	if got := env.countUsers(t); got != 1 {
		t.Fatalf("%d users after rollback, want 1", got)
	}
	if got := env.countRealUsers(t); got != 0 {
		t.Fatalf("%d real users after rollback, want 0", got)
	}
	defaultUser, err := env.queries.GetAppUserByUsername(context.Background(), managerTestDefaultUsername)
	if err != nil {
		t.Fatalf("load default user: %v", err)
	}
	if defaultUser.DisabledAt.Valid {
		t.Fatal("default account was disabled despite the rollback")
	}
	if !env.sessionExists(t, defaultLogin.Token) {
		t.Fatal("default session was deleted despite the rollback")
	}
	if _, err := env.manager.ResolveSession(context.Background(), defaultLogin.Token); err != nil {
		t.Fatalf("default session no longer resolves: %v", err)
	}
}

func TestPruneExpiredSessions(t *testing.T) {
	env := newManagerEnv(t)
	user := env.createUser(t, managerTestRealUsername, managerTestRealPassword, false)

	now := env.clock.Now()
	createSession := func(token string, lastSeenAt, expiresAt time.Time) {
		t.Helper()
		if err := env.queries.CreateAppSession(context.Background(), dbgen.CreateAppSessionParams{
			TokenHash:  HashSessionToken(token),
			UserID:     user.ID,
			CsrfToken:  "csrf-" + token,
			CreatedAt:  now.Add(-time.Hour),
			LastSeenAt: lastSeenAt,
			ExpiresAt:  expiresAt,
		}); err != nil {
			t.Fatalf("create session %q: %v", token, err)
		}
	}

	createSession("active-recent", now.Add(-time.Minute), now.Add(time.Hour))
	createSession("active-now", now, now.Add(2*time.Hour))
	createSession("idle-expired", now.Add(-SessionIdleTimeout-time.Second), now.Add(time.Hour))
	createSession("absolute-expired", now, now.Add(-time.Second))

	if err := env.manager.PruneExpiredSessions(context.Background()); err != nil {
		t.Fatalf("prune sessions: %v", err)
	}

	if got := env.countSessions(t); got != 2 {
		t.Fatalf("%d sessions after prune, want 2", got)
	}
	for _, token := range []string{"active-recent", "active-now"} {
		if !env.sessionExists(t, token) {
			t.Fatalf("active session %q was pruned", token)
		}
	}
	for _, token := range []string{"idle-expired", "absolute-expired"} {
		if env.sessionExists(t, token) {
			t.Fatalf("expired session %q was not pruned", token)
		}
	}
}
