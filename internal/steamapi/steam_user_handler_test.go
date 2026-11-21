package steamapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func newSteamUserTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	return newTestClient(t, handler, nil, nil)
}

func TestGetPlayerSummariesDecodesResponse(t *testing.T) {
	const steamID = "76561198000000001"
	client := newSteamUserTestClient(t, func(writer http.ResponseWriter, request *http.Request) {
		if got, want := request.URL.Path, "/ISteamUser/GetPlayerSummaries/v2/"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		query := request.URL.Query()
		if got := query.Get("steamids"); got != steamID {
			t.Errorf("steamids = %q, want %q", got, steamID)
		}
		_, _ = io.WriteString(writer, `{
			"response": {
				"players": [
					{
						"steamid": "`+steamID+`",
						"personaname": "Gabe",
						"profileurl": "https://steamcommunity.com/id/gaben/",
						"avatar": "https://cdn.cloudflare.steamstatic.com/steamcommunity/public/images/avatars/gabe.jpg",
						"avatarmedium": "https://cdn.cloudflare.steamstatic.com/steamcommunity/public/images/avatars/gabe_medium.jpg",
						"avatarfull": "https://cdn.cloudflare.steamstatic.com/steamcommunity/public/images/avatars/gabe_full.jpg",
						"lastlogoff": 1700000000
					}
				]
			}
		}`)
	})

	result, err := client.GetPlayerSummaries(context.Background(), []string{steamID})
	if err != nil {
		t.Fatalf("GetPlayerSummaries: %v", err)
	}
	if len(result.Response.Players) != 1 {
		t.Fatalf("players = %+v, want 1", result.Response.Players)
	}

	player := result.Response.Players[0]
	if player.SteamID != steamID {
		t.Errorf("steamid = %q, want %q", player.SteamID, steamID)
	}
	if player.PersonaName != "Gabe" {
		t.Errorf("personaname = %q, want Gabe", player.PersonaName)
	}
	if player.ProfileURL != "https://steamcommunity.com/id/gaben/" {
		t.Errorf("profileurl = %q", player.ProfileURL)
	}
	if !strings.HasSuffix(player.Avatar, "gabe.jpg") {
		t.Errorf("avatar = %q, want the small avatar URL", player.Avatar)
	}
	if !strings.HasSuffix(player.AvatarMedium, "gabe_medium.jpg") {
		t.Errorf("avatarmedium = %q, want the medium avatar URL", player.AvatarMedium)
	}
	if !strings.HasSuffix(player.AvatarFull, "gabe_full.jpg") {
		t.Errorf("avatarfull = %q, want the full avatar URL", player.AvatarFull)
	}
}

// An unknown Steam ID returns a response without players rather than an error.
// Callers decide that an empty list means "not found".
func TestGetPlayerSummariesMissingProfileIsNotAnError(t *testing.T) {
	client := newSteamUserTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"response":{}}`)
	})

	result, err := client.GetPlayerSummaries(context.Background(), []string{"76561198000000001"})
	if err != nil {
		t.Fatalf("GetPlayerSummaries: %v", err)
	}
	if len(result.Response.Players) != 0 {
		t.Fatalf("players = %+v, want none", result.Response.Players)
	}
}

// IDs are trimmed, normalized, deduplicated, and sorted so the same set of
// users always produces one request and one cache entry.
func TestGetPlayerSummariesNormalizesDeduplicatesAndSortsIDs(t *testing.T) {
	const first = "76561198000000001"
	const second = "76561198000000002"
	const third = "76561198000000003"

	var steamIDs []string
	cache := newFakeCache()
	client := newTestClient(t, func(writer http.ResponseWriter, request *http.Request) {
		steamIDs = strings.Split(request.URL.Query().Get("steamids"), ",")
		_, _ = io.WriteString(writer, `{"response":{"players":[]}}`)
	}, cache, nil)

	if _, err := client.GetPlayerSummaries(
		context.Background(),
		[]string{"  " + third + "  ", first, "0" + second, third, first},
	); err != nil {
		t.Fatalf("GetPlayerSummaries: %v", err)
	}

	want := []string{first, second, third}
	if strings.Join(steamIDs, ",") != strings.Join(want, ",") {
		t.Fatalf("steamids = %v, want %v", steamIDs, want)
	}
	entries := cache.recordedEntries()
	if len(entries) != 1 {
		t.Fatalf("recorded %d entries, want 1", len(entries))
	}
	if wantKey := "v2:player_summaries:" + strings.Join(want, ","); entries[0].key != wantKey {
		t.Errorf("cache key = %q, want %q", entries[0].key, wantKey)
	}
}

func TestGetPlayerSummariesRejectsBlankSteamIDs(t *testing.T) {
	client := newSteamUserTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		t.Error("HTTP request made for blank Steam IDs")
		writer.WriteHeader(http.StatusInternalServerError)
	})

	tests := []struct {
		name     string
		steamIDs []string
	}{
		{name: "none", steamIDs: nil},
		{name: "empty slice", steamIDs: []string{}},
		{name: "whitespace", steamIDs: []string{"  "}},
		{name: "blank among valid", steamIDs: []string{"76561198000000001", ""}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := client.GetPlayerSummaries(context.Background(), test.steamIDs); err == nil {
				t.Fatalf("expected an error for %v", test.steamIDs)
			}
		})
	}
}

func TestGetPlayerSummariesRateLimited(t *testing.T) {
	client := newSteamUserTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "9")
		writer.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := client.GetPlayerSummaries(context.Background(), []string{"76561198000000001"})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("error = %v, want ErrRateLimited", err)
	}
	var rateLimitError *RateLimitError
	if !errors.As(err, &rateLimitError) {
		t.Fatalf("error = %v, want *RateLimitError", err)
	}
	if rateLimitError.RetryAfter != 9*time.Second {
		t.Fatalf("retry after = %v, want 9s", rateLimitError.RetryAfter)
	}
}
