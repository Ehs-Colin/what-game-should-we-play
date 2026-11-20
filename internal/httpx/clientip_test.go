package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPKeyWithTrustedProxies(t *testing.T) {
	tests := []struct {
		name         string
		hops         int
		forwardedFor string
		want         string
	}{
		{name: "zero hops ignores forwarded header", hops: 0, forwardedFor: "203.0.113.7", want: "192.0.2.10"},
		{name: "single proxy uses last entry", hops: 1, forwardedFor: "203.0.113.7", want: "203.0.113.7"},
		{name: "client spoofed entries are ignored", hops: 1, forwardedFor: "198.51.100.99, 203.0.113.7", want: "203.0.113.7"},
		{name: "chained proxies", hops: 2, forwardedFor: "203.0.113.7, 198.51.100.9", want: "203.0.113.7"},
		{name: "whitespace is trimmed", hops: 1, forwardedFor: "203.0.113.7 , 198.51.100.9", want: "198.51.100.9"},
		{name: "missing header falls back to peer", hops: 1, forwardedFor: "", want: "192.0.2.10"},
		{name: "too few entries falls back to peer", hops: 2, forwardedFor: "203.0.113.7", want: "192.0.2.10"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/api/dashboard", nil)
			request.RemoteAddr = "192.0.2.10:54321"
			if test.forwardedFor != "" {
				request.Header.Set("X-Forwarded-For", test.forwardedFor)
			}

			key := ClientIPKeyWithTrustedProxies(test.hops)
			if got := key(request); got != test.want {
				t.Fatalf("client IP = %q, want %q", got, test.want)
			}
		})
	}
}
