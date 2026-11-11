package steamapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const (
	IPlayerServiceURL = "https://api.steampowered.com/IPlayerService/"
)

// type OwnedGamesRequest struct {
// 	Key                    string `json:"key"`
// 	SteamID                string `json:"steamid"`
// 	IncludeAppinfo         bool   `json:"include_appinfo"`
// 	IncludePlayedFreeGames bool   `json:"include_played_free_games"`
// 	AppidsFilter           []int  `json:"appids_filter"`
// 	IncludeFreeSub         bool   `json:"include_free_sub"`
// 	SkipUnvettedApps       bool   `json:"skip_unvetted_apps,omitempty"`
// 	Language               string `json:"language"`
// 	IncludeExtendedAppInfo bool   `json:"include_extended_appinfo"`
// 	IncludeFamilyLicenses  bool   `json:"include_family_licenses"`
// 	IncludeAllFreeGames    bool   `json:"include_all_free_games"`
// }

type OwnedGamesResponse struct {
	Response struct {
		Games []Games `json:"games"`
	} `json:"response"`
}

type Games struct {
	AppID int `json:"appid"`
}

// GetOwnedGames returns the apps a user owns. A private profile or empty
// library has no "games" field; that is returned as an empty list.
func (client *Client) GetOwnedGames(ctx context.Context, steamId int64) (*OwnedGamesResponse, error) {
	url, err := url.Parse(IPlayerServiceURL + "GetOwnedGames/v1/")
	if err != nil {
		return nil, fmt.Errorf("Failed to parse GetOwnedGames URL: %v", err)
	}
	values := url.Query()
	values.Add("key", client.steamApiKey)
	values.Add("steamid", strconv.FormatInt(steamId, 10))
	url.RawQuery = values.Encode()

	fmt.Printf("Getting owned games from URL: %s\n", url.String())
	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create GetOwnedGames request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to send GetOwnedGames request: %v", err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, "GetOwnedGames"); err != nil {
		return nil, err
	}
	var result OwnedGamesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("Failed to decode GetOwnedGames response: %v", err)
	}
	if result.Response.Games == nil {
		result.Response.Games = []Games{}
	}
	return &result, nil
}
