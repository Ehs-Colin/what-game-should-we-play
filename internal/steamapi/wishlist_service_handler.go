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
	requestURL, err := url.Parse(IWishlistServiceURL + "GetWishlist/v1/")
	if err != nil {
		return nil, err
	}
	values := requestURL.Query()
	values.Add("steamid", steamID)
	requestURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL.String(), nil)
	if err != nil {
		return nil, err
	}

	var result GetWishlistResponse
	err = client.fetch(
		ctx,
		"v1:wishlist:"+normalizeSteamID(steamID),
		req,
		func(resp *http.Response) error {
			return checkWishlistStatus(resp, "GetWishlistItems")
		},
		func(body []byte) error {
			if err := json.Unmarshal(body, &result); err != nil {
				return err
			}
			if result.Response.Items == nil {
				result.Response.Items = []WishlistItem{}
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return result.Response.Items, nil
}
