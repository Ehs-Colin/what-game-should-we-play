package steamapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func newTestClient(t *testing.T, handler http.HandlerFunc, cache Cache, limiter *rate.Limiter) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	client := NewClient(time.Second, "", cache, limiter)
	client.httpClient.Transport = rewriteTransport{target: target}
	return &client
}

type recordedCacheEntry struct {
	key  string
	body []byte
}

type fakeCache struct {
	mu        sync.Mutex
	bodies    map[string][]byte
	fresh     map[string]bool
	loadErr   error
	recordErr error
	loadKeys  []string
	recorded  []recordedCacheEntry
}

func newFakeCache() *fakeCache {
	return &fakeCache{
		bodies: map[string][]byte{},
		fresh:  map[string]bool{},
	}
}

func (cache *fakeCache) set(key string, body []byte, fresh bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.bodies[key] = body
	cache.fresh[key] = fresh
}

func (cache *fakeCache) Load(_ context.Context, key string, _ time.Time) ([]byte, bool, bool, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.loadKeys = append(cache.loadKeys, key)
	if cache.loadErr != nil {
		return nil, false, false, cache.loadErr
	}
	body, found := cache.bodies[key]
	if !found {
		return nil, false, false, nil
	}
	return body, cache.fresh[key], true, nil
}

func (cache *fakeCache) Record(_ context.Context, key string, body []byte, _ time.Time) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.recorded = append(cache.recorded, recordedCacheEntry{
		key:  key,
		body: append([]byte(nil), body...),
	})
	if cache.recordErr != nil {
		return cache.recordErr
	}
	cache.bodies[key] = body
	cache.fresh[key] = true
	return nil
}

func (cache *fakeCache) recordedEntries() []recordedCacheEntry {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return append([]recordedCacheEntry(nil), cache.recorded...)
}

func TestClientCacheKeys(t *testing.T) {
	const steamID = "76561198000000001"
	tests := []struct {
		name     string
		response string
		wantKey  string
		call     func(*Client) error
	}{
		{
			name:     "app detail",
			response: `{"440":{"success":true,"data":{"steam_appid":440}}}`,
			wantKey:  "v1:appdetails:440",
			call: func(client *Client) error {
				_, err := client.GetAppDetail(context.Background(), 440)
				return err
			},
		},
		{
			name:     "owned games",
			response: `{"response":{"games":[{"appid":440}]}}`,
			wantKey:  "v1:owned_games:" + steamID,
			call: func(client *Client) error {
				_, err := client.GetOwnedGames(context.Background(), 76561198000000001)
				return err
			},
		},
		{
			name:     "wishlist",
			response: `{"response":{"items":[]}}`,
			wantKey:  "v1:wishlist:" + steamID,
			call: func(client *Client) error {
				_, err := client.GetWishlistItems(context.Background(), steamID)
				return err
			},
		},
		{
			name:     "vanity",
			response: `{"response":{"success":1,"steamid":"` + steamID + `"}}`,
			wantKey:  "v1:vanity:myslug",
			call: func(client *Client) error {
				_, err := client.ResolveVanityURL(context.Background(), " MySlug ")
				return err
			},
		},
		{
			name:     "player summaries",
			response: `{"response":{"players":[{"steamid":"` + steamID + `","personaname":"Gabe"}]}}`,
			wantKey:  "v2:player_summaries:" + steamID,
			call: func(client *Client) error {
				_, err := client.GetPlayerSummaries(context.Background(), []string{steamID})
				return err
			},
		},
		{
			name:     "groups",
			response: `{"response":{"groups":[]}}`,
			wantKey:  "v1:groups:" + steamID,
			call: func(client *Client) error {
				_, err := client.GetUserGroupList(context.Background(), steamID)
				return err
			},
		},
		{
			name:     "friends",
			response: `{"response":{"friends":[]}}`,
			wantKey:  "v1:friends:" + steamID,
			call: func(client *Client) error {
				_, err := client.GetFriendList(context.Background(), steamID)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cache := newFakeCache()
			client := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(writer, test.response)
			}, cache, nil)

			if err := test.call(client); err != nil {
				t.Fatalf("call: %v", err)
			}
			entries := cache.recordedEntries()
			if len(entries) != 1 {
				t.Fatalf("recorded %d entries, want 1", len(entries))
			}
			if entries[0].key != test.wantKey {
				t.Errorf("cache key = %q, want %q", entries[0].key, test.wantKey)
			}
			if string(entries[0].body) != test.response {
				t.Errorf("recorded body = %q, want %q", entries[0].body, test.response)
			}
		})
	}
}

func TestClientNormalizesSteamIDInCacheKeys(t *testing.T) {
	const steamID = "76561198000000001"
	tests := []struct {
		name     string
		response string
		wantKey  string
		call     func(*Client) error
	}{
		{
			name:     "wishlist",
			response: `{"response":{"items":[]}}`,
			wantKey:  "v1:wishlist:" + steamID,
			call: func(client *Client) error {
				_, err := client.GetWishlistItems(context.Background(), "076561198000000001")
				return err
			},
		},
		{
			name:     "groups",
			response: `{"response":{"groups":[]}}`,
			wantKey:  "v1:groups:" + steamID,
			call: func(client *Client) error {
				_, err := client.GetUserGroupList(context.Background(), "076561198000000001")
				return err
			},
		},
		{
			name:     "friends",
			response: `{"response":{"friends":[]}}`,
			wantKey:  "v1:friends:" + steamID,
			call: func(client *Client) error {
				_, err := client.GetFriendList(context.Background(), "076561198000000001")
				return err
			},
		},
		{
			name:     "player summaries",
			response: `{"response":{"players":[]}}`,
			wantKey:  "v2:player_summaries:" + steamID,
			call: func(client *Client) error {
				_, err := client.GetPlayerSummaries(context.Background(), []string{" 076561198000000001 "})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cache := newFakeCache()
			client := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(writer, test.response)
			}, cache, nil)

			if err := test.call(client); err != nil {
				t.Fatalf("call: %v", err)
			}
			entries := cache.recordedEntries()
			if len(entries) != 1 {
				t.Fatalf("recorded %d entries, want 1", len(entries))
			}
			if entries[0].key != test.wantKey {
				t.Errorf("cache key = %q, want %q", entries[0].key, test.wantKey)
			}
		})
	}
}

func TestClientVanityNoMatchIsCached(t *testing.T) {
	const body = `{"response":{"success":42,"message":"No match"}}`
	cache := newFakeCache()
	client := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, body)
	}, cache, nil)

	result, err := client.ResolveVanityURL(context.Background(), "missing")
	if err != nil {
		t.Fatalf("ResolveVanityURL: %v", err)
	}
	if result.Response.Success != 42 {
		t.Errorf("success = %d, want 42", result.Response.Success)
	}
	entries := cache.recordedEntries()
	if len(entries) != 1 {
		t.Fatalf("recorded %d entries, want 1", len(entries))
	}
	if want := "v1:vanity:missing"; entries[0].key != want {
		t.Errorf("cache key = %q, want %q", entries[0].key, want)
	}
	if string(entries[0].body) != body {
		t.Errorf("recorded body = %q, want %q", entries[0].body, body)
	}
}

func TestClientFreshCacheHitSkipsHTTPAndLimiter(t *testing.T) {
	const body = `{"440":{"success":true,"data":{"steam_appid":440,"name":"Team Fortress 2"}}}`
	cache := newFakeCache()
	cache.set("v1:appdetails:440", []byte(body), true)
	// A zero-burst limiter fails immediately if the client waits on it.
	limiter := rate.NewLimiter(rate.Every(time.Hour), 0)
	client := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		t.Error("HTTP request made for a fresh cache hit")
		writer.WriteHeader(http.StatusInternalServerError)
	}, cache, limiter)

	detail, err := client.GetAppDetail(context.Background(), 440)
	if err != nil {
		t.Fatalf("GetAppDetail: %v", err)
	}
	if detail.Data == nil || detail.Data.Name != "Team Fortress 2" {
		t.Fatalf("detail = %+v, want cached Team Fortress 2", detail)
	}
	if entries := cache.recordedEntries(); len(entries) != 0 {
		t.Errorf("recorded %d entries on a fresh cache hit, want 0", len(entries))
	}
}

func TestClientCacheMissFetchesAndRecords(t *testing.T) {
	const body = `{"440":{"success":true,"data":{"steam_appid":440}}}`
	tests := []struct {
		name    string
		prepare func(*fakeCache)
	}{
		{name: "missing", prepare: func(*fakeCache) {}},
		{
			name: "stale",
			prepare: func(cache *fakeCache) {
				cache.set("v1:appdetails:440", []byte(`{"440":{"success":false}}`), false)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cache := newFakeCache()
			test.prepare(cache)
			client := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(writer, body)
			}, cache, nil)

			if _, err := client.GetAppDetail(context.Background(), 440); err != nil {
				t.Fatalf("GetAppDetail: %v", err)
			}
			entries := cache.recordedEntries()
			if len(entries) != 1 {
				t.Fatalf("recorded %d entries, want 1", len(entries))
			}
			if string(entries[0].body) != body {
				t.Errorf("recorded body = %q, want %q", entries[0].body, body)
			}
		})
	}
}

func TestClientCacheLoadErrorFetches(t *testing.T) {
	cache := newFakeCache()
	cache.loadErr = errors.New("load failed")
	client := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"440":{"success":true,"data":{"steam_appid":440}}}`)
	}, cache, nil)

	if _, err := client.GetAppDetail(context.Background(), 440); err != nil {
		t.Fatalf("GetAppDetail: %v", err)
	}
	if entries := cache.recordedEntries(); len(entries) != 1 {
		t.Errorf("recorded %d entries, want 1", len(entries))
	}
}

func TestClientCacheRecordErrorStillReturnsDecodedResponse(t *testing.T) {
	cache := newFakeCache()
	cache.recordErr = errors.New("record failed")
	client := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"440":{"success":true,"data":{"steam_appid":440}}}`)
	}, cache, nil)

	detail, err := client.GetAppDetail(context.Background(), 440)
	if err != nil {
		t.Fatalf("GetAppDetail: %v", err)
	}
	if detail.Data == nil || detail.Data.AppID != 440 {
		t.Errorf("detail = %+v, want app 440", detail)
	}
}

func TestClientCorruptCachedBodyFallsThrough(t *testing.T) {
	const body = `{"440":{"success":true,"data":{"steam_appid":440}}}`
	cache := newFakeCache()
	cache.set("v1:appdetails:440", []byte("not json"), true)
	client := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, body)
	}, cache, nil)

	if _, err := client.GetAppDetail(context.Background(), 440); err != nil {
		t.Fatalf("GetAppDetail: %v", err)
	}
	entries := cache.recordedEntries()
	if len(entries) != 1 {
		t.Fatalf("recorded %d entries, want 1", len(entries))
	}
	if string(entries[0].body) != body {
		t.Errorf("recorded body = %q, want %q", entries[0].body, body)
	}
}

func TestClientDoesNotRecordFailures(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		call    func(*Client) error
		wantErr error
	}{
		{
			name:   "app detail rate limited",
			status: http.StatusTooManyRequests,
			call: func(client *Client) error {
				_, err := client.GetAppDetail(context.Background(), 440)
				return err
			},
			wantErr: ErrRateLimited,
		},
		{
			name:   "app detail server error",
			status: http.StatusInternalServerError,
			call: func(client *Client) error {
				_, err := client.GetAppDetail(context.Background(), 440)
				return err
			},
		},
		{
			name:   "wishlist unauthorized",
			status: http.StatusUnauthorized,
			call: func(client *Client) error {
				_, err := client.GetWishlistItems(context.Background(), "76561198000000001")
				return err
			},
			wantErr: ErrWishlistPrivate,
		},
		{
			name:   "wishlist forbidden",
			status: http.StatusForbidden,
			call: func(client *Client) error {
				_, err := client.GetWishlistItems(context.Background(), "76561198000000001")
				return err
			},
			wantErr: ErrWishlistPrivate,
		},
		{
			name:   "app not found",
			status: http.StatusOK,
			body:   `{"440":{"success":false}}`,
			call: func(client *Client) error {
				_, err := client.GetAppDetail(context.Background(), 440)
				return err
			},
			wantErr: ErrAppNotFound,
		},
		{
			name:   "player summaries server error",
			status: http.StatusInternalServerError,
			call: func(client *Client) error {
				_, err := client.GetPlayerSummaries(context.Background(), []string{"76561198000000001"})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cache := newFakeCache()
			client := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, test.body)
			}, cache, nil)

			err := test.call(client)
			if err == nil {
				t.Fatal("call returned nil, want error")
			}
			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Errorf("error = %v, want %v", err, test.wantErr)
			}
			if entries := cache.recordedEntries(); len(entries) != 0 {
				t.Errorf("recorded %d entries for a failure, want 0", len(entries))
			}
		})
	}
}
