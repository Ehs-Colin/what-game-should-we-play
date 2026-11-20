package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/auth"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbtest"
)

const (
	authTestDefaultUsername = "admin"
	authTestDefaultPassword = "default-password-123"
	authTestRealUsername    = "player"
	authTestRealPassword    = "real-password-456"
	authTestClientIP        = "192.0.2.10"
)

type authHandlerEnv struct {
	db      *sql.DB
	queries *dbgen.Queries
	manager *auth.Manager
	handler *Handler
	mux     *http.ServeMux
}

func newAuthHandlerEnv(t *testing.T) *authHandlerEnv {
	t.Helper()
	db := dbtest.Open(t)
	manager := auth.NewManager(db, nil)
	if err := manager.EnsureDefaultUser(
		context.Background(),
		authTestDefaultUsername,
		authTestDefaultPassword,
	); err != nil {
		t.Fatalf("ensure default user: %v", err)
	}

	handler := NewHandler(
		NewStore(db, "", defaultResolver()),
		nil,
		nil,
		manager,
		false,
		NewLoginThrottle(),
		func(*http.Request) string { return authTestClientIP },
	)
	env := &authHandlerEnv{
		db:      db,
		queries: dbgen.New(db),
		manager: manager,
		handler: handler,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", handler.Login)
	mux.HandleFunc("POST /api/auth/logout", handler.Logout)
	mux.HandleFunc("GET /api/auth/session", handler.Session)
	mux.HandleFunc("POST /api/auth/account", env.withPrincipal(handler.CreateAccount))
	env.mux = mux
	return env
}

// withPrincipal mirrors the production middleware far enough for handler
// tests: it resolves the session cookie and stores the principal on the
// request context. CSRF checks are covered by the httpserver middleware tests.
func (env *authHandlerEnv) withPrincipal(next http.HandlerFunc) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if cookie, err := request.Cookie(auth.SessionCookieName); err == nil {
			if principal, err := env.manager.ResolveSession(request.Context(), cookie.Value); err == nil {
				request = request.WithContext(auth.WithPrincipal(request.Context(), principal))
			}
		}
		next(responseWriter, request)
	}
}

func (env *authHandlerEnv) do(
	t *testing.T,
	method string,
	target string,
	body string,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()
	var payload io.Reader
	if body != "" {
		payload = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, target, payload)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	env.mux.ServeHTTP(recorder, request)
	return recorder
}

func (env *authHandlerEnv) login(
	t *testing.T,
	username string,
	password string,
) (*http.Cookie, authLoginResponse) {
	t.Helper()
	recorder := env.do(
		t,
		http.MethodPost,
		"/api/auth/login",
		fmt.Sprintf(`{"username":%q,"password":%q}`, username, password),
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	return sessionCookieFrom(t, recorder), decodeLoginResponse(t, recorder)
}

func (env *authHandlerEnv) createUserDirect(
	t *testing.T,
	username string,
	password string,
	isDefault bool,
) dbgen.AppUser {
	t.Helper()
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user, err := env.queries.CreateAppUser(context.Background(), dbgen.CreateAppUserParams{
		Username:     username,
		PasswordHash: passwordHash,
		IsDefault:    isDefault,
		Now:          time.Now(),
	})
	if err != nil {
		t.Fatalf("create user %q: %v", username, err)
	}
	return user
}

// promoteDefault runs the same manager flow as a successful account creation
// and returns the real account's session.
func (env *authHandlerEnv) promoteDefault(t *testing.T) auth.LoginResult {
	t.Helper()
	defaultLogin, err := env.manager.Login(
		context.Background(),
		authTestDefaultUsername,
		authTestDefaultPassword,
	)
	if err != nil {
		t.Fatalf("login default: %v", err)
	}
	realLogin, err := env.manager.CreateRealAccount(
		context.Background(),
		defaultLogin.Principal,
		authTestDefaultPassword,
		authTestRealUsername,
		authTestRealPassword,
	)
	if err != nil {
		t.Fatalf("create real account: %v", err)
	}
	return realLogin
}

func (env *authHandlerEnv) countRealUsers(t *testing.T) int {
	t.Helper()
	count, err := env.queries.CountRealAppUsers(context.Background())
	if err != nil {
		t.Fatalf("count real users: %v", err)
	}
	return int(count)
}

func (env *authHandlerEnv) passwordHash(t *testing.T, username string) string {
	t.Helper()
	user, err := env.queries.GetAppUserByUsername(context.Background(), username)
	if err != nil {
		t.Fatalf("load user %q: %v", username, err)
	}
	return user.PasswordHash
}

func sessionCookieFrom(t *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	cookie := findSessionCookie(recorder)
	if cookie == nil {
		t.Fatalf("response did not set %s", auth.SessionCookieName)
	}
	return cookie
}

func findSessionCookie(recorder *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == auth.SessionCookieName {
			return cookie
		}
	}
	return nil
}

func decodeLoginResponse(t *testing.T, recorder *httptest.ResponseRecorder) authLoginResponse {
	t.Helper()
	var body authLoginResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode login response %q: %v", recorder.Body.String(), err)
	}
	return body
}

func decodeSessionResponse(t *testing.T, recorder *httptest.ResponseRecorder) authSessionAuthenticatedResponse {
	t.Helper()
	var body authSessionAuthenticatedResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode session response %q: %v", recorder.Body.String(), err)
	}
	return body
}

func decodeAnonymousSession(t *testing.T, recorder *httptest.ResponseRecorder) authSessionAnonymousResponse {
	t.Helper()
	var body authSessionAnonymousResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode session response %q: %v", recorder.Body.String(), err)
	}
	return body
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response %q: %v", recorder.Body.String(), err)
	}
	return body.Error
}

func accountRequestBody(defaultPassword, username, password string) string {
	return fmt.Sprintf(
		`{"defaultPassword":%q,"username":%q,"password":%q}`,
		defaultPassword,
		username,
		password,
	)
}

func principalFromLogin(body authLoginResponse) auth.Principal {
	return auth.Principal{
		UserID:            body.User.ID,
		Username:          body.User.Username,
		IsDefault:         body.User.IsDefault,
		MustCreateAccount: body.MustCreateAccount,
		CSRFToken:         body.CSRFToken,
	}
}

func (env *authHandlerEnv) createAccountAs(
	t *testing.T,
	principal auth.Principal,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/account",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(auth.WithPrincipal(request.Context(), principal))
	recorder := httptest.NewRecorder()
	env.handler.CreateAccount(recorder, request)
	return recorder
}

func TestAuthLoginSuccess(t *testing.T) {
	env := newAuthHandlerEnv(t)

	recorder := env.do(
		t,
		http.MethodPost,
		"/api/auth/login",
		fmt.Sprintf(
			`{"username":%q,"password":%q}`,
			authTestDefaultUsername,
			authTestDefaultPassword,
		),
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}

	cookie := sessionCookieFrom(t, recorder)
	if cookie.Value == "" {
		t.Fatal("session cookie has an empty value")
	}
	if cookie.Path != "/" {
		t.Fatalf("cookie path = %q, want /", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Fatal("session cookie is not HttpOnly")
	}
	if cookie.Secure {
		t.Fatal("session cookie is Secure even though the test origin is HTTP")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie SameSite = %v, want Lax", cookie.SameSite)
	}
	if want := int(auth.SessionAbsoluteTimeout.Seconds()); cookie.MaxAge != want {
		t.Fatalf("cookie MaxAge = %d, want %d", cookie.MaxAge, want)
	}

	body := decodeLoginResponse(t, recorder)
	if body.User.ID == 0 || body.User.Username != authTestDefaultUsername {
		t.Fatalf("user = %+v", body.User)
	}
	if !body.User.IsDefault || !body.MustCreateAccount {
		t.Fatalf("mustCreateAccount = %v, isDefault = %v; want both true", body.MustCreateAccount, body.User.IsDefault)
	}
	if body.CSRFToken == "" {
		t.Fatal("login returned an empty CSRF token")
	}

	passwordHash := env.passwordHash(t, authTestDefaultUsername)
	responseBody := recorder.Body.String()
	if strings.Contains(responseBody, passwordHash) || strings.Contains(responseBody, "argon2id") {
		t.Fatalf("login response leaked the password hash: %s", responseBody)
	}
	if strings.Contains(responseBody, cookie.Value) {
		t.Fatalf("login response leaked the raw session token: %s", responseBody)
	}
}

func TestAuthLoginInvalidCredentials(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{"wrong password", authTestDefaultUsername, "wrong-password-123"},
		{"unknown username", "nobody", authTestDefaultPassword},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := newAuthHandlerEnv(t)
			recorder := env.do(
				t,
				http.MethodPost,
				"/api/auth/login",
				fmt.Sprintf(`{"username":%q,"password":%q}`, test.username, test.password),
			)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (%s)", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder); got != "invalid username or password" {
				t.Fatalf("error = %q, want the generic credentials message", got)
			}
			if cookie := findSessionCookie(recorder); cookie != nil {
				t.Fatalf("failed login set a session cookie: %+v", cookie)
			}
		})
	}
}

func TestAuthLoginDisabledDefaultAccount(t *testing.T) {
	env := newAuthHandlerEnv(t)
	env.promoteDefault(t)

	recorder := env.do(
		t,
		http.MethodPost,
		"/api/auth/login",
		fmt.Sprintf(
			`{"username":%q,"password":%q}`,
			authTestDefaultUsername,
			authTestDefaultPassword,
		),
	)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (%s)", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder); got != "invalid username or password" {
		t.Fatalf("error = %q, want the generic credentials message", got)
	}
}

func TestAuthLoginThrottle(t *testing.T) {
	env := newAuthHandlerEnv(t)
	body := fmt.Sprintf(
		`{"username":%q,"password":"wrong-password-123"}`,
		authTestDefaultUsername,
	)

	for attempt := 1; attempt <= 5; attempt++ {
		recorder := env.do(t, http.MethodPost, "/api/auth/login", body)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401 (%s)", attempt, recorder.Code, recorder.Body.String())
		}
	}

	recorder := env.do(t, http.MethodPost, "/api/auth/login", body)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (%s)", recorder.Code, recorder.Body.String())
	}
	retryAfter := recorder.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Fatal("429 response is missing Retry-After")
	}
	seconds, err := strconv.Atoi(retryAfter)
	if err != nil || seconds <= 0 {
		t.Fatalf("Retry-After = %q, want a positive number of seconds", retryAfter)
	}
	if got := decodeError(t, recorder); got != "too many login attempts" {
		t.Fatalf("error = %q, want the throttle message", got)
	}

	recorder = env.do(
		t,
		http.MethodPost,
		"/api/auth/login",
		fmt.Sprintf(
			`{"username":%q,"password":%q}`,
			authTestDefaultUsername,
			authTestDefaultPassword,
		),
	)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("correct password after throttling: status = %d, want 429", recorder.Code)
	}
}

func TestAuthSessionAnonymous(t *testing.T) {
	env := newAuthHandlerEnv(t)

	recorder := env.do(t, http.MethodGet, "/api/auth/session", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	if body := decodeAnonymousSession(t, recorder); body.Authenticated {
		t.Fatal("anonymous session reports authenticated")
	}
	if strings.Contains(recorder.Body.String(), "csrfToken") {
		t.Fatalf("anonymous session leaked a CSRF token: %s", recorder.Body.String())
	}

	recorder = env.do(
		t,
		http.MethodGet,
		"/api/auth/session",
		"",
		&http.Cookie{Name: auth.SessionCookieName, Value: "invalid-session-token"},
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("invalid session status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	if body := decodeAnonymousSession(t, recorder); body.Authenticated {
		t.Fatal("invalid session reports authenticated")
	}
}

func TestAuthSessionAuthenticated(t *testing.T) {
	env := newAuthHandlerEnv(t)
	cookie, loginBody := env.login(t, authTestDefaultUsername, authTestDefaultPassword)

	recorder := env.do(t, http.MethodGet, "/api/auth/session", "", cookie)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	body := decodeSessionResponse(t, recorder)
	if !body.Authenticated {
		t.Fatal("session is not authenticated")
	}
	if body.User != loginBody.User {
		t.Fatalf("user = %+v, want %+v", body.User, loginBody.User)
	}
	if body.CSRFToken != loginBody.CSRFToken {
		t.Fatalf("CSRF token = %q, want %q", body.CSRFToken, loginBody.CSRFToken)
	}
	if !body.MustCreateAccount {
		t.Fatal("mustCreateAccount = false, want true")
	}
	if strings.Contains(recorder.Body.String(), cookie.Value) {
		t.Fatalf("session response leaked the raw session token: %s", recorder.Body.String())
	}
}

func TestAuthCreateAccount(t *testing.T) {
	env := newAuthHandlerEnv(t)
	cookie, _ := env.login(t, authTestDefaultUsername, authTestDefaultPassword)

	recorder := env.do(
		t,
		http.MethodPost,
		"/api/auth/account",
		accountRequestBody(
			authTestDefaultPassword,
			authTestRealUsername,
			authTestRealPassword,
		),
		cookie,
	)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", recorder.Code, recorder.Body.String())
	}

	newCookie := sessionCookieFrom(t, recorder)
	if newCookie.Value == "" || newCookie.Value == cookie.Value {
		t.Fatal("account creation did not rotate the session token")
	}
	body := decodeLoginResponse(t, recorder)
	if body.User.Username != authTestRealUsername || body.User.IsDefault {
		t.Fatalf("user = %+v, want the real account", body.User)
	}
	if body.MustCreateAccount {
		t.Fatal("mustCreateAccount = true for the real account")
	}
	if body.CSRFToken == "" {
		t.Fatal("account creation returned an empty CSRF token")
	}

	oldSession := env.do(t, http.MethodGet, "/api/auth/session", "", cookie)
	if decodeAnonymousSession(t, oldSession).Authenticated {
		t.Fatal("default session survived account creation")
	}
	newSession := env.do(t, http.MethodGet, "/api/auth/session", "", newCookie)
	sessionBody := decodeSessionResponse(t, newSession)
	if !sessionBody.Authenticated || sessionBody.User.Username != authTestRealUsername {
		t.Fatalf("new session = %+v, want the real account", sessionBody)
	}

	if got := env.countRealUsers(t); got != 1 {
		t.Fatalf("%d real users, want 1", got)
	}
	defaultUser, err := env.queries.GetAppUserByUsername(context.Background(), authTestDefaultUsername)
	if err != nil {
		t.Fatalf("load default user: %v", err)
	}
	if !defaultUser.DisabledAt.Valid {
		t.Fatal("default account was not disabled")
	}

	recorder = env.do(
		t,
		http.MethodPost,
		"/api/auth/login",
		fmt.Sprintf(
			`{"username":%q,"password":%q}`,
			authTestDefaultUsername,
			authTestDefaultPassword,
		),
	)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("default login after promotion: status = %d, want 401", recorder.Code)
	}
}

func TestAuthCreateAccountRejectsWrongDefaultPassword(t *testing.T) {
	env := newAuthHandlerEnv(t)
	cookie, _ := env.login(t, authTestDefaultUsername, authTestDefaultPassword)

	recorder := env.do(
		t,
		http.MethodPost,
		"/api/auth/account",
		accountRequestBody(
			"wrong-default-password",
			authTestRealUsername,
			authTestRealPassword,
		),
		cookie,
	)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder); got != "default password is incorrect" {
		t.Fatalf("error = %q, want the incorrect-default-password message", got)
	}

	if got := env.countRealUsers(t); got != 0 {
		t.Fatalf("%d real users, want 0", got)
	}
	sessionRecorder := env.do(t, http.MethodGet, "/api/auth/session", "", cookie)
	if !decodeSessionResponse(t, sessionRecorder).Authenticated {
		t.Fatal("default session no longer works after a rejected attempt")
	}
}

func TestAuthCreateAccountRejectsNonDefaultCaller(t *testing.T) {
	env := newAuthHandlerEnv(t)
	realLogin := env.promoteDefault(t)

	recorder := env.do(
		t,
		http.MethodPost,
		"/api/auth/account",
		accountRequestBody(
			authTestRealPassword,
			"another-player",
			"another-password-789",
		),
		&http.Cookie{Name: auth.SessionCookieName, Value: realLogin.Token},
	)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder); got != "account creation required" {
		t.Fatalf("error = %q, want the account creation message", got)
	}
}

func TestAuthCreateAccountRejectsSecondRealAccount(t *testing.T) {
	env := newAuthHandlerEnv(t)
	_, loginBody := env.login(t, authTestDefaultUsername, authTestDefaultPassword)
	env.createUserDirect(t, authTestRealUsername, authTestRealPassword, false)

	recorder := env.createAccountAs(
		t,
		principalFromLogin(loginBody),
		accountRequestBody(
			authTestDefaultPassword,
			"another-player",
			"another-password-789",
		),
	)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (%s)", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder); got != "a real account already exists" {
		t.Fatalf("error = %q, want the existing-account message", got)
	}
	if got := env.countRealUsers(t); got != 1 {
		t.Fatalf("%d real users, want 1", got)
	}
}

func TestAuthCreateAccountRejectsDuplicateUsername(t *testing.T) {
	env := newAuthHandlerEnv(t)
	cookie, _ := env.login(t, authTestDefaultUsername, authTestDefaultPassword)

	recorder := env.do(
		t,
		http.MethodPost,
		"/api/auth/account",
		accountRequestBody(
			authTestDefaultPassword,
			strings.ToUpper(authTestDefaultUsername),
			authTestRealPassword,
		),
		cookie,
	)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (%s)", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder); got != "username is already taken" {
		t.Fatalf("error = %q, want the duplicate-username message", got)
	}
}

func TestAuthLogout(t *testing.T) {
	env := newAuthHandlerEnv(t)
	cookie, _ := env.login(t, authTestDefaultUsername, authTestDefaultPassword)

	recorder := env.do(t, http.MethodPost, "/api/auth/logout", "", cookie)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (%s)", recorder.Code, recorder.Body.String())
	}
	cleared := sessionCookieFrom(t, recorder)
	if cleared.Value != "" {
		t.Fatalf("cleared cookie value = %q, want empty", cleared.Value)
	}
	if cleared.MaxAge >= 0 {
		t.Fatalf("cleared cookie MaxAge = %d, want a negative value", cleared.MaxAge)
	}
	if !cleared.HttpOnly || cleared.SameSite != http.SameSiteLaxMode || cleared.Path != "/" {
		t.Fatalf("cleared cookie attributes = %+v", cleared)
	}

	sessionRecorder := env.do(t, http.MethodGet, "/api/auth/session", "", cookie)
	if decodeAnonymousSession(t, sessionRecorder).Authenticated {
		t.Fatal("session still authenticates after logout")
	}

	if recorder := env.do(t, http.MethodPost, "/api/auth/logout", "", cookie); recorder.Code != http.StatusNoContent {
		t.Fatalf("second logout status = %d, want 204", recorder.Code)
	}
	if recorder := env.do(t, http.MethodPost, "/api/auth/logout", ""); recorder.Code != http.StatusNoContent {
		t.Fatalf("anonymous logout status = %d, want 204", recorder.Code)
	}
}

func TestAuthResponsesHideSecrets(t *testing.T) {
	env := newAuthHandlerEnv(t)
	cookie, _ := env.login(t, authTestDefaultUsername, authTestDefaultPassword)
	passwordHash := env.passwordHash(t, authTestDefaultUsername)

	loginRecorder := env.do(
		t,
		http.MethodPost,
		"/api/auth/login",
		fmt.Sprintf(
			`{"username":%q,"password":%q}`,
			authTestDefaultUsername,
			authTestDefaultPassword,
		),
	)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginRecorder.Code)
	}
	loginCookie := sessionCookieFrom(t, loginRecorder)

	sessionRecorder := env.do(t, http.MethodGet, "/api/auth/session", "", cookie)
	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("session status = %d, want 200", sessionRecorder.Code)
	}

	accountRecorder := env.do(
		t,
		http.MethodPost,
		"/api/auth/account",
		accountRequestBody(
			authTestDefaultPassword,
			authTestRealUsername,
			authTestRealPassword,
		),
		cookie,
	)
	if accountRecorder.Code != http.StatusCreated {
		t.Fatalf("account status = %d, want 201 (%s)", accountRecorder.Code, accountRecorder.Body.String())
	}
	accountCookie := sessionCookieFrom(t, accountRecorder)

	secrets := []string{
		passwordHash,
		"argon2id",
		cookie.Value,
		loginCookie.Value,
		accountCookie.Value,
	}
	for name, recorder := range map[string]*httptest.ResponseRecorder{
		"login":   loginRecorder,
		"session": sessionRecorder,
		"account": accountRecorder,
	} {
		body := recorder.Body.String()
		for _, secret := range secrets {
			if secret != "" && strings.Contains(body, secret) {
				t.Fatalf("%s response contains a secret %q: %s", name, secret, body)
			}
		}
	}
}
