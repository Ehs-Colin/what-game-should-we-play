package steamapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

func (client *Client) ResolveVanityURL(ctx context.Context, vanityUrl string) (*VanityUrl, error) {
	url, err := url.Parse(ISteamUserURL + "ResolveVanityURL/v1/")
	if err != nil {
		return nil, err
	}
	values := url.Query()
	values.Add("key", client.steamApiKey)
	values.Add("vanityurl", vanityUrl)
	url.RawQuery = values.Encode()

	fmt.Printf("Steam API Key: %s\n", client.steamApiKey)
	fmt.Printf("Resolving vanity URL: %s\n", url.String())

	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result VanityUrl
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (client *Client) GetUserGroupList(ctx context.Context, steamId string) (*GetUserGroupListResponse, error) {
	//NOTES::Only seems to show group Id, not name.  May not be useful after all
	url, err := url.Parse(ISteamUserURL + "GetUserGroupList/v1/")
	if err != nil {
		return nil, err
	}
	values := url.Query()
	values.Add("key", client.steamApiKey)
	values.Add("steamid", steamId)
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
	var result GetUserGroupListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	//TODO::Write to db
	return &result, nil
}

func (client *Client) GetFriendList(ctx context.Context, steamId string) (*GetFriendListResponse, error) {
	url, err := url.Parse(ISteamUserURL + "GetFriendList/v1/")
	if err != nil {
		return nil, err
	}
	values := url.Query()
	values.Add("key", client.steamApiKey)
	values.Add("steamid", steamId)
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
	var result GetFriendListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	//TODO::Write to db
	return &result, nil
}
