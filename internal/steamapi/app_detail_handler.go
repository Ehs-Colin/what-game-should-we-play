package steamapi

import (
	"encoding/json"
	"fmt"
	"io"
)

func (client *Client) GetAppDetail(appID int) (*AppDetail, error) {
	url := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%d", appID)
	resp, err := client.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var appDetail AppDetail
	if err := json.Unmarshal(body, &appDetail); err != nil {
		return nil, err
	}
	if !appDetail.Success {
		return nil, fmt.Errorf("app detail not found")
	}
	return &appDetail, nil
}
