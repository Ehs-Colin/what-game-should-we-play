package steamapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// rewriteTransport sends requests for the real Steam host to a test server.
type rewriteTransport struct {
	target *url.URL
}

func (transport rewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.URL.Scheme = transport.target.Scheme
	clone.URL.Host = transport.target.Host
	clone.Host = transport.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}

func newWishlistTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	return newTestClient(t, handler, nil, nil)
}

func TestGetWishlistItemsDecodesResponse(t *testing.T) {
	client := newWishlistTestClient(t, func(writer http.ResponseWriter, request *http.Request) {
		if got := request.URL.Query().Get("steamid"); got != "76561198000000001" {
			t.Errorf("steamid = %q", got)
		}
		_, _ = io.WriteString(writer, `{
			"response": {
				"items": [
					{"appid": 440, "date_added": 1700000000},
					{"appid": 570, "date_added": 1700000001}
				]
			}
		}`)
	})

	items, err := client.GetWishlistItems(context.Background(), "76561198000000001")
	if err != nil {
		t.Fatalf("GetWishlistItems: %v", err)
	}
	want := []WishlistItem{
		{AppID: 440, DateAdded: 1700000000},
		{AppID: 570, DateAdded: 1700000001},
	}
	if len(items) != len(want) {
		t.Fatalf("items = %+v, want %+v", items, want)
	}
	for i := range want {
		if items[i] != want[i] {
			t.Fatalf("items[%d] = %+v, want %+v", i, items[i], want[i])
		}
	}
}

func TestGetWishlistItemsEmptyResponse(t *testing.T) {
	client := newWishlistTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"response":{}}`)
	})

	items, err := client.GetWishlistItems(context.Background(), "76561198000000001")
	if err != nil {
		t.Fatalf("GetWishlistItems: %v", err)
	}
	if items == nil {
		t.Fatalf("items should be an empty slice, not nil")
	}
	if len(items) != 0 {
		t.Fatalf("items = %+v, want empty", items)
	}
}

func TestGetWishlistItemsPrivate(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := newWishlistTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(status)
			})

			_, err := client.GetWishlistItems(context.Background(), "76561198000000001")
			if !errors.Is(err, ErrWishlistPrivate) {
				t.Fatalf("error = %v, want ErrWishlistPrivate", err)
			}
			if errors.Is(err, ErrRateLimited) {
				t.Fatalf("private wishlist should not be reported as rate limited: %v", err)
			}
		})
	}
}

func TestGetWishlistItemsRateLimited(t *testing.T) {
	client := newWishlistTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "7")
		writer.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := client.GetWishlistItems(context.Background(), "76561198000000001")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("error = %v, want ErrRateLimited", err)
	}
	var rateLimitError *RateLimitError
	if !errors.As(err, &rateLimitError) {
		t.Fatalf("error = %v, want *RateLimitError", err)
	}
	if rateLimitError.RetryAfter != 7*time.Second {
		t.Fatalf("retry after = %v, want 7s", rateLimitError.RetryAfter)
	}
}

func TestGetWishlistItemsOtherStatus(t *testing.T) {
	client := newWishlistTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.GetWishlistItems(context.Background(), "76561198000000001")
	if err == nil {
		t.Fatalf("expected an error for a 500 response")
	}
	if errors.Is(err, ErrWishlistPrivate) || errors.Is(err, ErrRateLimited) {
		t.Fatalf("unexpected error type: %v", err)
	}
}
