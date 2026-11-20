package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/auth"
)

type fakeSessionResolver struct {
	principal auth.Principal
	err       error
}

func (fake fakeSessionResolver) ResolveSession(context.Context, string) (auth.Principal, error) {
	return fake.principal, fake.err
}

func testOptions(publicDirectory string) Options {
	return Options{
		PublicDirectory: publicDirectory,
		Sessions: fakeSessionResolver{principal: auth.Principal{
			UserID:    1,
			Username:  "tester",
			CSRFToken: "test-csrf-token",
		}},
	}
}

func authenticatedRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "test-session"})
	return request
}

func TestServesStaticFiles(t *testing.T) {
	publicDirectory := t.TempDir()
	const (
		indexContent  = "<!doctype html><title>static test</title>"
		scriptContent = "console.log('static test');"
		styleContent  = "body { color: black; }"
	)
	for name, content := range map[string]string{
		"index.html": indexContent,
		"app.js":     scriptContent,
		"styles.css": styleContent,
	} {
		if err := os.WriteFile(filepath.Join(publicDirectory, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	application, err := New(nil, nil, testOptions(publicDirectory))
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	defer application.Close()

	tests := []struct {
		name          string
		path          string
		body          string
		contentType   string
		authenticated bool
	}{
		{name: "index", path: "/", body: indexContent, contentType: "text/html", authenticated: true},
		{name: "script without session", path: "/app.js", body: scriptContent, contentType: "text/javascript"},
		{name: "style without session", path: "/styles.css", body: styleContent, contentType: "text/css"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			var request *http.Request
			if test.authenticated {
				request = authenticatedRequest(http.MethodGet, test.path)
			} else {
				request = httptest.NewRequest(http.MethodGet, test.path, nil)
			}
			application.Handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
			}
			if recorder.Body.String() != test.body {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), test.body)
			}
			if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, test.contentType) {
				t.Fatalf("Content-Type = %q, want it to contain %q", contentType, test.contentType)
			}
		})
	}

	// Unknown API paths fall through to the file server, which must not serve
	// the page for them. Authentication is required before routing, so the same
	// path is a 401 without a session and a 404 with one.
	anonymousRecorder := httptest.NewRecorder()
	application.Handler.ServeHTTP(
		anonymousRecorder,
		httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil),
	)
	if anonymousRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous unknown API path: status = %d, want 401 (%s)", anonymousRecorder.Code, anonymousRecorder.Body.String())
	}

	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodGet, "/api/does-not-exist")
	application.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown API path: status = %d, want 404 (%s)", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), indexContent) {
		t.Fatalf("unknown API path served the index page: %s", recorder.Body.String())
	}
}

func TestServesAuthPagesWithoutSession(t *testing.T) {
	publicDirectory := t.TempDir()
	const indexContent = "<!doctype html><title>auth shell test</title>"
	if err := os.WriteFile(filepath.Join(publicDirectory, "index.html"), []byte(indexContent), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	application, err := New(nil, nil, testOptions(publicDirectory))
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	defer application.Close()

	for _, path := range []string{"/login", "/create-account"} {
		t.Run("page "+path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			application.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
			}
			if recorder.Body.String() != indexContent {
				t.Fatalf("body = %q, want the SPA shell", recorder.Body.String())
			}
		})
	}

	for path, wantTarget := range map[string]string{
		"/":       "/login?next=%2F",
		"/import": "/login?next=%2Fimport",
	} {
		t.Run("root redirect "+path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			application.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			if recorder.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302 (%s)", recorder.Code, recorder.Body.String())
			}
			if location := recorder.Header().Get("Location"); location != wantTarget {
				t.Fatalf("Location = %q, want %q", location, wantTarget)
			}
		})
	}
}

func TestServesSPAFallbackForClientRoutes(t *testing.T) {
	publicDirectory := t.TempDir()
	const (
		indexContent = "<!doctype html><title>spa test</title>"
		styleContent = "body { color: black; }"
	)
	if err := os.WriteFile(filepath.Join(publicDirectory, "index.html"), []byte(indexContent), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.Mkdir(filepath.Join(publicDirectory, "assets"), 0o755); err != nil {
		t.Fatalf("make assets directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDirectory, "assets", "site.css"), []byte(styleContent), 0o644); err != nil {
		t.Fatalf("write assets/site.css: %v", err)
	}

	application, err := New(nil, nil, testOptions(publicDirectory))
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	defer application.Close()

	routes := []string{
		"/import",
		"/search",
		"/user",
		"/user?name=gaben&list=owned&page=1&pageSize=25",
		"/import/",
		"/some/deep/route",
	}
	for _, route := range routes {
		t.Run("route "+route, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := authenticatedRequest(http.MethodGet, route)
			application.Handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
			}
			if recorder.Body.String() != indexContent {
				t.Fatalf("body = %q, want the SPA shell", recorder.Body.String())
			}
			if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
				t.Fatalf("Content-Type = %q, want text/html", contentType)
			}
		})
	}

	t.Run("existing nested asset", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := authenticatedRequest(http.MethodGet, "/assets/site.css")
		application.Handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
		}
		if recorder.Body.String() != styleContent {
			t.Fatalf("body = %q, want %q", recorder.Body.String(), styleContent)
		}
	})

	missingAssets := []string{"/missing.js", "/missing.css", "/missing.png", "/nested/missing.json"}
	for _, asset := range missingAssets {
		t.Run("missing asset "+asset, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := authenticatedRequest(http.MethodGet, asset)
			application.Handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404 (%s)", recorder.Code, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), indexContent) {
				t.Fatalf("missing asset served the index page: %s", recorder.Body.String())
			}
		})
	}

	apiPaths := []string{"/api", "/api/does-not-exist"}
	for _, apiPath := range apiPaths {
		t.Run("api "+apiPath, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := authenticatedRequest(http.MethodGet, apiPath)
			application.Handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404 (%s)", recorder.Code, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), indexContent) {
				t.Fatalf("unknown API path served the index page: %s", recorder.Body.String())
			}
		})
	}
}
