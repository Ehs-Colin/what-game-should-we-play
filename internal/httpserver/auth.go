package httpserver

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/auth"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/logging"
)

// SessionResolver looks up the principal for a raw session token. The auth
// manager satisfies it in production; tests can provide a fake.
type SessionResolver interface {
	ResolveSession(ctx context.Context, token string) (auth.Principal, error)
}

// isPublicRequest reports whether the request may run without a session.
func isPublicRequest(request *http.Request) bool {
	path := request.URL.Path
	switch {
	case request.Method == http.MethodGet && path == "/health":
		return true
	case request.Method == http.MethodGet && (path == "/login" || path == "/create-account"):
		return true
	case request.Method == http.MethodGet && (path == "/styles.css" || path == "/app.js" || path == "/favicon.ico"):
		return true
	case request.Method == http.MethodPost && path == "/api/auth/login":
		return true
	case request.Method == http.MethodGet && path == "/api/auth/session":
		return true
	case request.Method == http.MethodPost && path == "/api/auth/logout":
		return true
	default:
		return false
	}
}

func isAPIRequest(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// requireAuth rejects anonymous requests and enforces the default account's
// forced account creation. It also checks the session CSRF token on
// state-changing requests.
func requireAuth(sessions SessionResolver, logger *logging.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			if isPublicRequest(request) {
				next.ServeHTTP(responseWriter, request)
				return
			}
			responseWriter.Header().Set("Cache-Control", "no-store")

			cookie, err := request.Cookie(auth.SessionCookieName)
			if err != nil || cookie.Value == "" || sessions == nil {
				rejectUnauthenticated(responseWriter, request)
				return
			}

			principal, err := sessions.ResolveSession(request.Context(), cookie.Value)
			if err != nil {
				if logger != nil && !errors.Is(err, auth.ErrNoSession) {
					_ = logger.Event("auth_session_resolve_failed", map[string]any{
						"path":  request.URL.Path,
						"error": err.Error(),
					})
				}
				rejectUnauthenticated(responseWriter, request)
				return
			}

			if principal.MustCreateAccount {
				accountCreationPath := request.Method == http.MethodGet && request.URL.Path == "/create-account"
				accountCreationAPI := request.Method == http.MethodPost && request.URL.Path == "/api/auth/account"
				if !accountCreationPath && !accountCreationAPI {
					rejectAccountCreation(responseWriter, request)
					return
				}
			}

			if !isSafeMethod(request.Method) {
				csrfToken := request.Header.Get("X-CSRF-Token")
				if subtle.ConstantTimeCompare([]byte(csrfToken), []byte(principal.CSRFToken)) != 1 {
					httpx.RespondWithError(responseWriter, http.StatusForbidden, "invalid CSRF token")
					return
				}
			}

			ctx := auth.WithPrincipal(request.Context(), principal)
			next.ServeHTTP(responseWriter, request.WithContext(ctx))
		})
	}
}

func rejectUnauthenticated(responseWriter http.ResponseWriter, request *http.Request) {
	if isAPIRequest(request.URL.Path) {
		httpx.RespondWithError(responseWriter, http.StatusUnauthorized, "authentication required")
		return
	}
	http.Redirect(responseWriter, request, loginRedirectTarget(request), http.StatusFound)
}

func rejectAccountCreation(responseWriter http.ResponseWriter, request *http.Request) {
	if isAPIRequest(request.URL.Path) {
		httpx.RespondWithError(responseWriter, http.StatusForbidden, "account creation required")
		return
	}
	http.Redirect(responseWriter, request, "/create-account", http.StatusFound)
}

func loginRedirectTarget(request *http.Request) string {
	return "/login?next=" + url.QueryEscape(request.URL.RequestURI())
}
