package steamapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

func (client *Client) GetWishlistItems(ctx context.Context, steamID string) ([]WishlistItem, error) {
	url, err := url.Parse(IWishlistServiceURL + "GetWishlist/v1/")
	if err != nil {
		return nil, err
	}
	values := url.Query()
	values.Add("steamid", steamID)
	url.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var items []WishlistItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}
