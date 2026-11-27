package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
)

// errSteamUserNotFound means Steam has no profile for the vanity name.
// Handlers map it to a 404; every other resolver error is a 500.
var errSteamUserNotFound = errors.New("steam user not found")

// resolveSteamPlayer maps a vanity name to the user's Steam ID and profile
// summary using read-only Steam calls. Steam's "no match" and an empty player
// list become errSteamUserNotFound; transport and API errors are wrapped for
// a 500. The lookup and insert handlers share it so the resolution rules stay
// in one place.
func (handler *Handler) resolveSteamPlayer(ctx context.Context, vanityURL string) (int64, steamapi.PlayerSummary, error) {
	resolved, err := handler.apiStore.steamClient.ResolveVanityURL(ctx, vanityURL)
	if err != nil {
		return 0, steamapi.PlayerSummary{}, fmt.Errorf("resolve vanity URL: %w", err)
	}
	// Steam returns success == 1 when the vanity URL resolves to a user.
	if resolved.Response.Success != 1 {
		return 0, steamapi.PlayerSummary{}, fmt.Errorf("%w: %q", errSteamUserNotFound, vanityURL)
	}
	steamID, err := strconv.ParseInt(resolved.Response.SteamID, 10, 64)
	if err != nil {
		return 0, steamapi.PlayerSummary{}, fmt.Errorf("parse resolved steam id %q: %w", resolved.Response.SteamID, err)
	}

	player, err := handler.fetchSteamPlayerByID(ctx, steamID)
	if err != nil {
		return 0, steamapi.PlayerSummary{}, err
	}
	return steamID, player, nil
}

// fetchSteamPlayerByID loads a profile directly by Steam ID with a read-only
// GetPlayerSummaries call, skipping vanity resolution for users without a
// custom URL. An empty player list is errSteamUserNotFound; the profile is
// validated before any import writes anything.
func (handler *Handler) fetchSteamPlayerByID(ctx context.Context, steamID int64) (steamapi.PlayerSummary, error) {
	summaries, err := handler.apiStore.steamClient.GetPlayerSummaries(ctx, []string{formatSteamID(steamID)})
	if err != nil {
		return steamapi.PlayerSummary{}, fmt.Errorf("get player summaries: %w", err)
	}
	// Steam returns an empty players list when the ID has no profile.
	if len(summaries.Response.Players) == 0 {
		return steamapi.PlayerSummary{}, fmt.Errorf("%w: %d", errSteamUserNotFound, steamID)
	}
	return summaries.Response.Players[0], nil
}
