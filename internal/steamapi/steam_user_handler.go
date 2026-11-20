package steamapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func (client *Client) ResolveVanityURL(ctx context.Context, vanityUrl string) (*VanityUrl, error) {
	requestURL, err := url.Parse(ISteamUserURL + "ResolveVanityURL/v1/")
	if err != nil {
		return nil, err
	}
	values := requestURL.Query()
	values.Add("key", client.steamApiKey)
	values.Add("vanityurl", vanityUrl)
	requestURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL.String(), nil)
	if err != nil {
		return nil, err
	}

	var result VanityUrl
	err = client.fetch(
		ctx,
		"v1:vanity:"+strings.ToLower(strings.TrimSpace(vanityUrl)),
		req,
		func(resp *http.Response) error {
			return checkStatus(resp, "ResolveVanityURL")
		},
		func(body []byte) error {
			return json.Unmarshal(body, &result)
		},
	)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (client *Client) GetUserGroupList(ctx context.Context, steamId string) (*GetUserGroupListResponse, error) {
	//NOTES::Only seems to show group Id, not name.  May not be useful after all
	requestURL, err := url.Parse(ISteamUserURL + "GetUserGroupList/v1/")
	if err != nil {
		return nil, err
	}
	values := requestURL.Query()
	values.Add("key", client.steamApiKey)
	values.Add("steamid", steamId)
	requestURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL.String(), nil)
	if err != nil {
		return nil, err
	}

	var result GetUserGroupListResponse
	err = client.fetch(
		ctx,
		"v1:groups:"+normalizeSteamID(steamId),
		req,
		func(resp *http.Response) error {
			return checkStatus(resp, "GetUserGroupList")
		},
		func(body []byte) error {
			return json.Unmarshal(body, &result)
		},
	)
	if err != nil {
		return nil, err
	}
	//TODO::Write to db
	return &result, nil
}

func (client *Client) GetFriendList(ctx context.Context, steamId string) (*GetFriendListResponse, error) {
	requestURL, err := url.Parse(ISteamUserURL + "GetFriendList/v1/")
	if err != nil {
		return nil, err
	}
	values := requestURL.Query()
	values.Add("key", client.steamApiKey)
	values.Add("steamid", steamId)
	requestURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL.String(), nil)
	if err != nil {
		return nil, err
	}

	var result GetFriendListResponse
	err = client.fetch(
		ctx,
		"v1:friends:"+normalizeSteamID(steamId),
		req,
		func(resp *http.Response) error {
			return checkStatus(resp, "GetFriendList")
		},
		func(body []byte) error {
			return json.Unmarshal(body, &result)
		},
	)
	if err != nil {
		return nil, err
	}
	//TODO::Write to db
	return &result, nil
}
