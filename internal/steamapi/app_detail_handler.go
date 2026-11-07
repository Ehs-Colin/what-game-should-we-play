package steamapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	SteamStoreURL = "https://store.steampowered.com/api/appdetails"
)

// ErrAppNotFound is returned when Steam has no details for an app, e.g. it was removed from the store.
var ErrAppNotFound = errors.New("app detail not found")

func (client *Client) GetAppDetail(ctx context.Context, appID int) (*AppDetail, error) {
	params := url.Values{}
	params.Set("appids", strconv.Itoa(appID))
	reqURL := SteamStoreURL + "?" + params.Encode()

	fmt.Printf("Getting details on App from URL: %v\n", reqURL)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create GetAppDetail request: %v", err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to send GetAppDetail request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GetAppDetail returned status %d", resp.StatusCode)
	}

	var result map[string]AppDetail
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode GetAppDetail response: %w", err)
	}

	appDetail, ok := result[strconv.Itoa(appID)]
	if !ok || !appDetail.Success || appDetail.Data == nil {
		return nil, fmt.Errorf("%w for appID %d", ErrAppNotFound, appID)
	}

	return &appDetail, nil
}

func (client *Client) GetAppDetails(ctx context.Context, ownedGames OwnedGamesResponse) (map[string]AppDetail, error) {
	games := ownedGames.Response.Games
	if len(games) == 0 {
		return map[string]AppDetail{}, nil
	}

	appIDs := make([]string, 0, len(games))
	for _, game := range games {
		appIDs = append(appIDs, strconv.Itoa(game.AppID))
	}

	params := url.Values{}
	params.Set("appids", strings.Join(appIDs, ","))
	if len(appIDs) > 0 {
		params.Set("filters", strings.Join(appIDs, ","))
	}
	reqURL := SteamStoreURL + "?" + params.Encode()

	fmt.Printf("Getting details on Apps from URL: %v\n", reqURL)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create GetAppDetails request: %v", err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to send GetAppDetails request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GetAppDetails returned status %d", resp.StatusCode)
	}

	var result map[string]AppDetail
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode GetAppDetails response: %w", err)
	}

	return result, nil
}
