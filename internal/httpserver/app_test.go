package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServesStaticFiles(t *testing.T) {
	publicDirectory := t.TempDir()
	const (
		indexContent  = "<!doctype html><title>static test</title>"
		scriptContent = "console.log('static test');"
	)
	for name, content := range map[string]string{
		"index.html": indexContent,
		"app.js":     scriptContent,
	} {
		if err := os.WriteFile(filepath.Join(publicDirectory, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	application, err := New(nil, nil, Options{PublicDirectory: publicDirectory})
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	defer application.Close()

	tests := []struct {
		name        string
		path        string
		body        string
		contentType string
	}{
		{name: "index", path: "/", body: indexContent, contentType: "text/html"},
		{name: "script", path: "/app.js", body: scriptContent, contentType: "text/javascript"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
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
	// the page for them.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil)
	application.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown API path: status = %d, want 404 (%s)", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), indexContent) {
		t.Fatalf("unknown API path served the index page: %s", recorder.Body.String())
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

	application, err := New(nil, nil, Options{PublicDirectory: publicDirectory})
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
			request := httptest.NewRequest(http.MethodGet, route, nil)
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
		request := httptest.NewRequest(http.MethodGet, "/assets/site.css", nil)
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
			request := httptest.NewRequest(http.MethodGet, asset, nil)
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
			request := httptest.NewRequest(http.MethodGet, apiPath, nil)
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
