package steamapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// GetWishlistItems returns the apps on a user's wishlist. A private or empty
// wishlist has no "items" field; that is returned as an empty list.
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
	if err := checkWishlistStatus(resp, "GetWishlistItems"); err != nil {
		return nil, err
	}
	var result GetWishlistResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Response.Items == nil {
		result.Response.Items = []WishlistItem{}
	}
	return result.Response.Items, nil
}
