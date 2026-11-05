package steamapi

import (
	"encoding/json"
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

func (client *Client) GetOwnedGames(steamId int64) (*OwnedGamesResponse, error) {
	url, err := url.Parse(ISteamUserURL + "GetOwnedGames/v1/")
	if err != nil {
		return nil, err
	}
	values := url.Query()
	values.Add("key", client.steamApiKey)
	values.Add("steamid", strconv.FormatInt(steamId, 10))
	url.RawQuery = values.Encode()

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result OwnedGamesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
