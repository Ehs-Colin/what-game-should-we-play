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
