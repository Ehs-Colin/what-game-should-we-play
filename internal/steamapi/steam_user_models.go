package steamapi

const (
	ISteamUserURL = "https://api.steampowered.com/ISteamUser/"
)

// type VanityUrlParam struct {
// 	Key       string `json:"key"`
// 	VanityUrl string `json:"vanityurl"`
// 	UrlType   string `json:"url_type"`
// }

type VanityUrl struct {
	Response VanityUrlResponse `json:"response"`
}

type VanityUrlResponse struct {
	Success int    `json:"success"`
	SteamID string `json:"steamid"`
}

type GetUserGroupListParam struct {
	Key     string `json:"key"`
	SteamID string `json:"steamid"`
}

type GetUserGroupListResponse struct {
	Response GetUserGroupListResponseData `json:"response"`
}

type GetUserGroupListResponseData struct {
	Groups []Group `json:"groups"`
}

type Group struct {
	GroupID int    `json:"groupid"`
	Name    string `json:"name"`
}

type GetPlayerSummariesResponse struct {
	Response GetPlayerSummariesResponseData `json:"response"`
}

type GetPlayerSummariesResponseData struct {
	Players []PlayerSummary `json:"players"`
}

// PlayerSummary holds the profile fields used by the app. Steam sends three
// avatar sizes; all are kept so callers can pick the resolution they need.
type PlayerSummary struct {
	SteamID      string `json:"steamid"`
	PersonaName  string `json:"personaname"`
	ProfileURL   string `json:"profileurl"`
	Avatar       string `json:"avatar"`
	AvatarMedium string `json:"avatarmedium"`
	AvatarFull   string `json:"avatarfull"`
}

type GetFriendListParam struct {
	Key          string `json:"key"`
	SteamID      string `json:"steamid"`
	Relationship string `json:"relationship"`
}

type GetFriendListResponse struct {
	Response GetFriendListResponseData `json:"response"`
}

type GetFriendListResponseData struct {
	Friends []Friend `json:"friends"`
}

type Friend struct {
	SteamID      string `json:"steamid"`
	Relationship string `json:"relationship"`
}
