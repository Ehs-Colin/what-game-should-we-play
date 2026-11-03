package steamapi

const (
	IPlayerServiceURL = "https://api.steampowered.com/IPlayerService/"
)

type GetOwnedGames struct {
	Key                    string `json:"key"`
	SteamID                string `json:"steamid"`
	IncludeAppinfo         bool   `json:"include_appinfo"`
	IncludePlayedFreeGames bool   `json:"include_played_free_games"`
	AppidsFilter           []int  `json:"appids_filter"`
	IncludeFreeSub         bool   `json:"include_free_sub"`
	SkipUnvettedApps       bool   `json:"skip_unvetted_apps,omitempty"`
	Language               string `json:"language"`
	IncludeExtendedAppInfo bool   `json:"include_extended_appinfo"`
	IncludeFamilyLicenses  bool   `json:"include_family_licenses"`
	IncludeAllFreeGames    bool   `json:"include_all_free_games"`
}

type GetRecentlyPlayedGames struct {
	Key     string `json:"key"`
	SteamID string `json:"steamid"`
	Count   int    `json:"count"`
}
