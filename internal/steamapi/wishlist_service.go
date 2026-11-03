package steamapi

const (
	WishlistServiceURL = "https://api.steampowered.com/IWishlistService/"
)

type GetWishlist struct {
	Steamid string `json:"steamid"`
}

type GetWishlistItemCount struct {
	Steamid string `json:"steamid"`
}

type GetWishlistItemsOnSale struct {
	Key string `json:"key"`
}
