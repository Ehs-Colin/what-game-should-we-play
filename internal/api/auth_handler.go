package api

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/auth"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
)

const maxAuthRequestBodyBytes = 4 << 10

type authUserResponse struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	IsDefault bool   `json:"isDefault"`
}

type authLoginResponse struct {
	User              authUserResponse `json:"user"`
	CSRFToken         string           `json:"csrfToken"`
	MustCreateAccount bool             `json:"mustCreateAccount"`
}

type authSessionAuthenticatedResponse struct {
	Authenticated     bool             `json:"authenticated"`
	User              authUserResponse `json:"user"`
	CSRFToken         string           `json:"csrfToken"`
	MustCreateAccount bool             `json:"mustCreateAccount"`
}

type authSessionAnonymousResponse struct {
	Authenticated bool `json:"authenticated"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type createAccountRequest struct {
	DefaultPassword string `json:"defaultPassword"`
	Username        string `json:"username"`
	Password        string `json:"password"`
}

func setSessionCookie(responseWriter http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	http.SetCookie(responseWriter, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(auth.SessionAbsoluteTimeout.Seconds()),
	})
}

func clearSessionCookie(responseWriter http.ResponseWriter, secure bool) {
	http.SetCookie(responseWriter, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
}

// Login handles POST /api/auth/login.
func (handler *Handler) Login(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Cache-Control", "no-store")

	if handler.auth == nil {
		handler.internalError(responseWriter, request, errors.New("auth service is not configured"))
		return
	}

	var body loginRequest
	if err := decodeAuthBody(responseWriter, request, &body); err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || body.Password == "" {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "username and password are required")
		return
	}

	ip := handler.clientIP(request)
	if handler.loginLimiter != nil {
		if retryAfter, blocked := handler.loginLimiter.Check(ip, body.Username); blocked {
			writeThrottled(responseWriter, retryAfter)
			return
		}
	}

	previousToken := sessionCookieValue(request)
	result, err := handler.auth.Login(request.Context(), body.Username, body.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			if handler.loginLimiter != nil {
				handler.loginLimiter.RecordFailure(ip, body.Username)
			}
			httpx.RespondWithError(responseWriter, http.StatusUnauthorized, "invalid username or password")
			return
		}
		handler.internalError(responseWriter, request, err)
		return
	}

	if handler.loginLimiter != nil {
		handler.loginLimiter.RecordSuccess(body.Username)
	}
	// Rotate any session referenced by the old cookie to prevent session
	// fixation.
	if previousToken != "" {
		if err := handler.auth.Logout(request.Context(), previousToken); err != nil {
			handler.logAuthEvent("auth_session_cleanup_failed", map[string]any{"error": err.Error()})
		}
	}

	setSessionCookie(responseWriter, result.Token, result.ExpiresAt, handler.cookieSecure)
	httpx.RespondWithJSON(responseWriter, http.StatusOK, loginResponse(result))
}

// Session handles GET /api/auth/session. It is public and always returns 200;
// an invalid or expired session is reported as unauthenticated.
func (handler *Handler) Session(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Cache-Control", "no-store")

	if handler.auth == nil {
		httpx.RespondWithJSON(responseWriter, http.StatusOK, authSessionAnonymousResponse{})
		return
	}
	token := sessionCookieValue(request)
	if token == "" {
		httpx.RespondWithJSON(responseWriter, http.StatusOK, authSessionAnonymousResponse{})
		return
	}

	principal, err := handler.auth.ResolveSession(request.Context(), token)
	if err != nil {
		if !errors.Is(err, auth.ErrNoSession) {
			handler.logAuthEvent("auth_session_resolve_failed", map[string]any{"error": err.Error()})
		}
		httpx.RespondWithJSON(responseWriter, http.StatusOK, authSessionAnonymousResponse{})
		return
	}

	httpx.RespondWithJSON(responseWriter, http.StatusOK, authSessionAuthenticatedResponse{
		Authenticated:     true,
		User:              authUser(principal),
		CSRFToken:         principal.CSRFToken,
		MustCreateAccount: principal.MustCreateAccount,
	})
}

// CreateAccount handles POST /api/auth/account for the signed-in default
// account.
func (handler *Handler) CreateAccount(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Cache-Control", "no-store")

	principal, ok := auth.PrincipalFromContext(request.Context())
	if !ok || !principal.IsDefault {
		httpx.RespondWithError(responseWriter, http.StatusForbidden, "account creation required")
		return
	}
	if handler.auth == nil {
		handler.internalError(responseWriter, request, errors.New("auth service is not configured"))
		return
	}

	var body createAccountRequest
	if err := decodeAuthBody(responseWriter, request, &body); err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.DefaultPassword == "" || body.Username == "" || body.Password == "" {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "all fields are required")
		return
	}

	result, err := handler.auth.CreateRealAccount(
		request.Context(),
		principal,
		body.DefaultPassword,
		body.Username,
		body.Password,
	)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			httpx.RespondWithError(responseWriter, http.StatusForbidden, "default password is incorrect")
		case errors.Is(err, auth.ErrAccountCreationRequired):
			httpx.RespondWithError(responseWriter, http.StatusForbidden, "account creation required")
		case errors.Is(err, auth.ErrRealAccountExists):
			httpx.RespondWithError(responseWriter, http.StatusConflict, "a real account already exists")
		case errors.Is(err, auth.ErrUsernameTaken):
			httpx.RespondWithError(responseWriter, http.StatusConflict, "username is already taken")
		case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrInvalidPassword):
			httpx.RespondWithError(responseWriter, http.StatusBadRequest, err.Error())
		default:
			handler.internalError(responseWriter, request, err)
		}
		return
	}

	setSessionCookie(responseWriter, result.Token, result.ExpiresAt, handler.cookieSecure)
	httpx.RespondWithJSON(responseWriter, http.StatusCreated, loginResponse(result))
}

// Logout handles POST /api/auth/logout. It is public and idempotent.
func (handler *Handler) Logout(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Cache-Control", "no-store")

	token := sessionCookieValue(request)
	if token != "" && handler.auth != nil {
		if err := handler.auth.Logout(request.Context(), token); err != nil {
			handler.logAuthEvent("auth_logout_failed", map[string]any{"error": err.Error()})
		}
	}
	clearSessionCookie(responseWriter, handler.cookieSecure)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func decodeAuthBody(responseWriter http.ResponseWriter, request *http.Request, destination any) error {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxAuthRequestBodyBytes)
	return json.NewDecoder(request.Body).Decode(destination)
}

func sessionCookieValue(request *http.Request) string {
	cookie, err := request.Cookie(auth.SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func loginResponse(result auth.LoginResult) authLoginResponse {
	return authLoginResponse{
		User:              authUser(result.Principal),
		CSRFToken:         result.Principal.CSRFToken,
		MustCreateAccount: result.Principal.MustCreateAccount,
	}
}

func authUser(principal auth.Principal) authUserResponse {
	return authUserResponse{
		ID:        principal.UserID,
		Username:  principal.Username,
		IsDefault: principal.IsDefault,
	}
}

func writeThrottled(responseWriter http.ResponseWriter, retryAfter time.Duration) {
	retryAfterSeconds := max(1, int(math.Ceil(retryAfter.Seconds())))
	responseWriter.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	httpx.RespondWithError(responseWriter, http.StatusTooManyRequests, "too many login attempts")
}

func (handler *Handler) logAuthEvent(eventName string, fields map[string]any) {
	if handler.logger == nil {
		return
	}
	_ = handler.logger.Event(eventName, fields)
}
