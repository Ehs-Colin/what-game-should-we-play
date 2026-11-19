package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateRequestOrigin(t *testing.T) {
	const appOrigin = "https://example.test"

	tests := []struct {
		name       string
		method     string
		origin     string
		referer    string
		wantStatus int
		wantNext   bool
	}{
		{name: "non-post passes through", method: http.MethodGet, wantStatus: http.StatusNoContent, wantNext: true},
		{name: "matching origin", method: http.MethodPost, origin: appOrigin, wantStatus: http.StatusNoContent, wantNext: true},
		{name: "matching referer", method: http.MethodPost, referer: appOrigin + "/some/page", wantStatus: http.StatusNoContent, wantNext: true},
		{name: "foreign origin", method: http.MethodPost, origin: "https://evil.test", wantStatus: http.StatusForbidden, wantNext: false},
		{name: "missing origin", method: http.MethodPost, wantStatus: http.StatusForbidden, wantNext: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calledNext := false
			next := http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
				calledNext = true
				responseWriter.WriteHeader(http.StatusNoContent)
			})
			handler := validateRequestOrigin(appOrigin)(next)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, "/api/steamuser", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.referer != "" {
				request.Header.Set("Referer", test.referer)
			}
			handler.ServeHTTP(recorder, request)

			if calledNext != test.wantNext {
				t.Fatalf("next called = %v, want %v", calledNext, test.wantNext)
			}
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if test.wantNext {
				return
			}
			if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
				t.Fatalf("Content-Type = %q, want text/html", contentType)
			}
			if body := recorder.Body.String(); !strings.Contains(body, "Forbidden") {
				t.Fatalf("body = %q, want it to contain the error page title", body)
			}
		})
	}
}

func TestRecoverPanics(t *testing.T) {
	t.Run("panic becomes an error page", func(t *testing.T) {
		handler := recoverPanics(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("database credentials leaked: hunter2")
		}))

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
		}
		if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
			t.Fatalf("Content-Type = %q, want text/html", contentType)
		}
		body := recorder.Body.String()
		if !strings.Contains(body, "Unhandled Error") {
			t.Fatalf("body = %q, want it to contain the error page title", body)
		}
		if strings.Contains(body, "hunter2") {
			t.Fatalf("body = %q, must not leak the panic value", body)
		}
	})

	t.Run("abort handler is re-panicked", func(t *testing.T) {
		handler := recoverPanics(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic(http.ErrAbortHandler)
		}))

		defer func() {
			if recovered := recover(); recovered != http.ErrAbortHandler {
				t.Fatalf("recovered = %v, want http.ErrAbortHandler", recovered)
			}
		}()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	})

	t.Run("no panic passes through", func(t *testing.T) {
		handler := recoverPanics(nil)(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
			responseWriter.WriteHeader(http.StatusNoContent)
		}))

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
		}
	})
}

func TestSearchThrottle(t *testing.T) {
	handler := SearchThrottle()(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}))

	for attempt := 1; attempt <= 5; attempt++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/steamusers/search", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("attempt %d status = %d, want %d", attempt, recorder.Code, http.StatusOK)
		}
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/steamusers/search", nil))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", contentType)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "Search Is Busy") {
		t.Fatalf("body = %q, want it to contain the error page title", body)
	}
}
