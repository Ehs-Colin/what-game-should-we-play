package steamapi

type AppDetail struct {
	Success bool `json:"success"`
	Data    struct {
		Name                string `json:"name"`
		SteamAppID          int    `json:"steam_appid"`
		DetailedDescription string `json:"detailed_description"`
		AboutTheGame        string `json:"about_the_game"`
		ShortDescription    string `json:"short_description"`
		HeaderImage         string `json:"header_image"`
		Website             string `json:"website"`
		PriceOverview       struct {
			Currency        string `json:"currency"`
			Initial         int    `json:"initial"`
			Final           int    `json:"final"`
			DiscountPercent int    `json:"discount_percent"`
			FinalFormatted  string `json:"final_formatted"`
		} `json:"price_overview"`
		Platforms struct {
			Windows bool `json:"windows"`
			Mac     bool `json:"mac"`
			Linux   bool `json:"linux"`
		} `json:"platforms"`
		Categories  []Category `json:"categories"`
		Genres      []Genre    `json:"genres"`
		ReleaseDate struct {
			ComingSoon bool   `json:"coming_soon"`
			Date       string `json:"date"`
		} `json:"release_date"`
	} `json:"data"`
}

type Category struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
}

type Genre struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
}
