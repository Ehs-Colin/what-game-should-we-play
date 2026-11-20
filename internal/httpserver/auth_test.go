package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/auth"
)

const (
	realSessionToken    = "real-session-token"
	defaultSessionToken = "default-session-token"
)

var (
	testRealPrincipal = auth.Principal{
		UserID:    11,
		Username:  "real-user",
		CSRFToken: "real-csrf-token",
	}
	testDefaultPrincipal = auth.Principal{
		UserID:            12,
		Username:          "default-user",
		IsDefault:         true,
		MustCreateAccount: true,
		CSRFToken:         "default-csrf-token",
	}
)

type tokenSessionResolver map[string]auth.Principal

func (resolver tokenSessionResolver) ResolveSession(
	_ context.Context,
	token string,
) (auth.Principal, error) {
	principal, ok := resolver[token]
	if !ok {
		return auth.Principal{}, auth.ErrNoSession
	}
	return principal, nil
}

func testSessionResolver() tokenSessionResolver {
	return tokenSessionResolver{
		realSessionToken:    testRealPrincipal,
		defaultSessionToken: testDefaultPrincipal,
	}
}

func serveRequireAuth(
	resolver SessionResolver,
	request *http.Request,
) (*httptest.ResponseRecorder, bool) {
	reached := false
	next := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		reached = true
		if principal, ok := auth.PrincipalFromContext(request.Context()); ok {
			responseWriter.Header().Set("X-Test-Principal", principal.Username)
		}
		responseWriter.WriteHeader(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	requireAuth(resolver, nil)(next).ServeHTTP(recorder, request)
	return recorder, reached
}

func middlewareRequest(method, target, token string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	if token != "" {
		request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	}
	return request
}

func TestRequireAuthAllowsPublicRequests(t *testing.T) {
	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/health"},
		{http.MethodGet, "/login"},
		{http.MethodGet, "/create-account"},
		{http.MethodGet, "/styles.css"},
		{http.MethodGet, "/app.js"},
		{http.MethodGet, "/favicon.ico"},
		{http.MethodGet, "/api/auth/session"},
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/api/auth/logout"},
	}
	for _, test := range paths {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			recorder, reached := serveRequireAuth(
				testSessionResolver(),
				middlewareRequest(test.method, test.path, ""),
			)
			if !reached {
				t.Fatalf("public request did not reach the handler (status %d)", recorder.Code)
			}
			if recorder.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204", recorder.Code)
			}
			if location := recorder.Header().Get("Location"); location != "" {
				t.Fatalf("public request redirected to %q", location)
			}
		})
	}
}

func TestRequireAuthAllowsPublicRequestsWithStaleCookie(t *testing.T) {
	recorder, reached := serveRequireAuth(
		testSessionResolver(),
		middlewareRequest(http.MethodGet, "/login", "expired-session-token"),
	)
	if !reached || recorder.Code != http.StatusNoContent {
		t.Fatalf("reached = %v, status = %d; want the public handler", reached, recorder.Code)
	}
}

func TestRequireAuthRejectsAnonymousRequests(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		token      string
		wantStatus int
		wantTarget string
		wantError  string
	}{
		{
			name:       "html root redirects",
			method:     http.MethodGet,
			target:     "/",
			wantStatus: http.StatusFound,
			wantTarget: "/login?next=%2F",
		},
		{
			name:       "html route keeps query",
			method:     http.MethodGet,
			target:     "/import?name=a%20b",
			wantStatus: http.StatusFound,
			wantTarget: "/login?next=" + url.QueryEscape("/import?name=a%20b"),
		},
		{
			name:       "api request returns json",
			method:     http.MethodGet,
			target:     "/api/dashboard",
			wantStatus: http.StatusUnauthorized,
			wantError:  "authentication required",
		},
		{
			name:       "api post returns json",
			method:     http.MethodPost,
			target:     "/api/common-games",
			wantStatus: http.StatusUnauthorized,
			wantError:  "authentication required",
		},
		{
			name:       "expired html session redirects",
			method:     http.MethodGet,
			target:     "/",
			token:      "expired-session-token",
			wantStatus: http.StatusFound,
			wantTarget: "/login?next=%2F",
		},
		{
			name:       "expired api session returns json",
			method:     http.MethodGet,
			target:     "/api/dashboard",
			token:      "expired-session-token",
			wantStatus: http.StatusUnauthorized,
			wantError:  "authentication required",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder, reached := serveRequireAuth(
				testSessionResolver(),
				middlewareRequest(test.method, test.target, test.token),
			)
			if reached {
				t.Fatal("anonymous request reached the handler")
			}
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", cacheControl)
			}

			if test.wantTarget != "" {
				if location := recorder.Header().Get("Location"); location != test.wantTarget {
					t.Fatalf("Location = %q, want %q", location, test.wantTarget)
				}
				return
			}

			if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
				t.Fatalf("Content-Type = %q, want application/json", contentType)
			}
			if location := recorder.Header().Get("Location"); location != "" {
				t.Fatalf("JSON response has a Location header %q", location)
			}
			if body := recorder.Body.String(); !strings.Contains(body, `"error":"`+test.wantError+`"`) {
				t.Fatalf("body = %q, want error %q", body, test.wantError)
			}
		})
	}
}

func TestRequireAuthAllowsRealSession(t *testing.T) {
	for _, target := range []string{"/", "/api/dashboard"} {
		t.Run(target, func(t *testing.T) {
			recorder, reached := serveRequireAuth(
				testSessionResolver(),
				middlewareRequest(http.MethodGet, target, realSessionToken),
			)
			if !reached {
				t.Fatalf("request did not reach the handler (status %d)", recorder.Code)
			}
			if recorder.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204", recorder.Code)
			}
			if principal := recorder.Header().Get("X-Test-Principal"); principal != testRealPrincipal.Username {
				t.Fatalf("principal = %q, want %q", principal, testRealPrincipal.Username)
			}
			if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", cacheControl)
			}
		})
	}
}

func TestRequireAuthForcesDefaultAccountSetup(t *testing.T) {
	t.Run("html redirects to create account", func(t *testing.T) {
		for _, target := range []string{"/", "/import"} {
			recorder, reached := serveRequireAuth(
				testSessionResolver(),
				middlewareRequest(http.MethodGet, target, defaultSessionToken),
			)
			if reached {
				t.Fatalf("default account reached %q", target)
			}
			if recorder.Code != http.StatusFound || recorder.Header().Get("Location") != "/create-account" {
				t.Fatalf("%q: status = %d, Location = %q; want 302 /create-account", target, recorder.Code, recorder.Header().Get("Location"))
			}
		}
	})

	t.Run("api returns account creation error", func(t *testing.T) {
		recorder, reached := serveRequireAuth(
			testSessionResolver(),
			middlewareRequest(http.MethodGet, "/api/dashboard", defaultSessionToken),
		)
		if reached {
			t.Fatal("default account reached the API handler")
		}
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403 (%s)", recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), `"error":"account creation required"`) {
			t.Fatalf("body = %q, want the account creation error", recorder.Body.String())
		}
	})

	t.Run("create account page is allowed", func(t *testing.T) {
		recorder, reached := serveRequireAuth(
			testSessionResolver(),
			middlewareRequest(http.MethodGet, "/create-account", defaultSessionToken),
		)
		if !reached || recorder.Code != http.StatusNoContent {
			t.Fatalf("reached = %v, status = %d; want the create-account page", reached, recorder.Code)
		}
	})

	t.Run("create account api is allowed with csrf", func(t *testing.T) {
		request := middlewareRequest(http.MethodPost, "/api/auth/account", defaultSessionToken)
		request.Header.Set("X-CSRF-Token", testDefaultPrincipal.CSRFToken)
		recorder, reached := serveRequireAuth(testSessionResolver(), request)
		if !reached || recorder.Code != http.StatusNoContent {
			t.Fatalf("reached = %v, status = %d; want the account creation handler", reached, recorder.Code)
		}
	})

	t.Run("create account api requires csrf", func(t *testing.T) {
		recorder, reached := serveRequireAuth(
			testSessionResolver(),
			middlewareRequest(http.MethodPost, "/api/auth/account", defaultSessionToken),
		)
		if reached {
			t.Fatal("account creation ran without a CSRF token")
		}
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", recorder.Code)
		}
		if !strings.Contains(recorder.Body.String(), `"error":"invalid CSRF token"`) {
			t.Fatalf("body = %q, want the CSRF error", recorder.Body.String())
		}
	})
}

func TestRequireAuthEnforcesCSRF(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		method  string
		target  string
		want    int
		reached bool
	}{
		{name: "safe method without token", token: realSessionToken, method: http.MethodGet, target: "/api/dashboard", want: http.StatusNoContent, reached: true},
		{name: "missing csrf token", token: realSessionToken, method: http.MethodPost, target: "/api/steamuser", want: http.StatusForbidden},
		{name: "wrong csrf token", token: realSessionToken, method: http.MethodPost, target: "/api/steamuser", want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := middlewareRequest(test.method, test.target, test.token)
			if test.name == "wrong csrf token" {
				request.Header.Set("X-CSRF-Token", "wrong-token")
			}
			recorder, reached := serveRequireAuth(testSessionResolver(), request)
			if reached != test.reached {
				t.Fatalf("reached = %v, want %v", reached, test.reached)
			}
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d (%s)", recorder.Code, test.want, recorder.Body.String())
			}
			if !test.reached && !strings.Contains(recorder.Body.String(), `"error":"invalid CSRF token"`) {
				t.Fatalf("body = %q, want the CSRF error", recorder.Body.String())
			}
		})
	}

	t.Run("valid csrf token", func(t *testing.T) {
		request := middlewareRequest(http.MethodPost, "/api/steamuser", realSessionToken)
		request.Header.Set("X-CSRF-Token", testRealPrincipal.CSRFToken)
		recorder, reached := serveRequireAuth(testSessionResolver(), request)
		if !reached || recorder.Code != http.StatusNoContent {
			t.Fatalf("reached = %v, status = %d; want the protected handler", reached, recorder.Code)
		}
	})
}
