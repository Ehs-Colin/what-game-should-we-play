package steamapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create GetAppDetail request: %v", err)
	}

	var detail *AppDetail
	err = client.fetch(
		ctx,
		"v1:appdetails:"+strconv.Itoa(appID),
		req,
		func(resp *http.Response) error {
			return checkStatus(resp, "GetAppDetail")
		},
		func(body []byte) error {
			var result map[string]AppDetail
			if err := json.Unmarshal(body, &result); err != nil {
				return fmt.Errorf("failed to decode GetAppDetail response: %w", err)
			}

			appDetail, ok := result[strconv.Itoa(appID)]
			if !ok || !appDetail.Success || appDetail.Data == nil {
				return fmt.Errorf("%w for appID %d", ErrAppNotFound, appID)
			}
			detail = &appDetail
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return detail, nil
}
