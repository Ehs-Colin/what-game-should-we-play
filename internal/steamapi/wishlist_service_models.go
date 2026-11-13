package steamapi

const (
	IWishlistServiceURL = "https://api.steampowered.com/IWishlistService/"
)

type GetWishlistRequest struct {
	SteamID string `json:"steamid"`
}

type GetWishlistResponse struct {
	Response GetWishlistResponseData `json:"response"`
}

type GetWishlistResponseData struct {
	Items []WishlistItem `json:"items"`
}

type WishlistItem struct {
	AppID     int `json:"appid"`
	DateAdded int `json:"date_added"`
}
