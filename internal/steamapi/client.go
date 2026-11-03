package steamapi

import (
	"net/http"
	"time"
)

type Client struct {
	cache       SteamCache
	httpClient  http.Client
	steamApiKey string
}

// NewClient -
func NewClient(timeout time.Duration, cacheInterval time.Duration, steamApiKey string) Client {
	return Client{
		cache: NewCache(cacheInterval),
		httpClient: http.Client{
			Timeout: timeout,
		},
		steamApiKey: steamApiKey,
	}
}
