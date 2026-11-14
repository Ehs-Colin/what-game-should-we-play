package steamapi

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// DefaultStoreInterval is the minimum delay between actual Steam store
// requests. Cache hits do not wait.
const DefaultStoreInterval = 1500 * time.Millisecond

type Client struct {
	cache       Cache
	httpClient  http.Client
	steamApiKey string
	limiter     *rate.Limiter
	now         func() time.Time
}

// NewClient -
func NewClient(timeout time.Duration, steamApiKey string, cache Cache, limiter *rate.Limiter) Client {
	return Client{
		cache: cache,
		httpClient: http.Client{
			Timeout: timeout,
		},
		steamApiKey: steamApiKey,
		limiter:     limiter,
		now:         time.Now,
	}
}

// fetch loads a fresh cached response when possible. On a miss it waits for
// the limiter, performs the request, checks the status, reads the body,
// decodes it, and records the raw body. Cache failures are best effort.
func (client *Client) fetch(
	ctx context.Context,
	key string,
	req *http.Request,
	check func(*http.Response) error,
	decode func([]byte) error,
) error {
	if client.cache != nil {
		body, fresh, found, err := client.cache.Load(ctx, key, client.now())
		if err == nil && found && fresh {
			if err := decode(body); err == nil {
				return nil
			}
			// Corrupt or incompatible cached body: fall through and refetch.
		}
		// Cache errors are logged by the store and treated as a miss.
	}

	if client.limiter != nil {
		if err := client.limiter.Wait(ctx); err != nil {
			return err
		}
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := check(resp); err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := decode(body); err != nil {
		return err
	}
	if client.cache != nil {
		_ = client.cache.Record(ctx, key, body, client.now())
	}
	return nil
}

func normalizeSteamID(steamID string) string {
	trimmed := strings.TrimSpace(steamID)
	if id, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return strconv.FormatInt(id, 10)
	}
	return trimmed
}
