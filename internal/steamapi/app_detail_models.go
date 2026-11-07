package steamapi

type AppDetail struct {
	Success bool `json:"success"`
	Data    *struct {
		Type  string `json:"type"`
		Name  string `json:"name"`
		AppID int    `json:"steam_appid"`
		// RequiredAge         int    `json:"required_age"`
		IsFree              bool   `json:"is_free"`
		DLC                 []int  `json:"dlc"`
		ControllerSupport   string `json:"controller_support"`
		DetailedDescription string `json:"detailed_description"`
		AboutTheGame        string `json:"about_the_game"`
		ShortDescription    string `json:"short_description"`
		Fullgame            struct {
			AppID int    `json:"appid"`
			Name  string `json:"name"`
		} `json:"fullgame"`
		SupportedLanguages string `json:"supported_languages"`
		Reviews            string `json:"reviews"`
		HeaderImage        string `json:"header_image"`
		Website            string `json:"website"`
		// PcRequirements     struct {
		// 	Minimum     string `json:"minimum"`
		// 	Recommended string `json:"recommended"`
		// } `json:"pc_requirements"`
		// MacRequirements struct {
		// 	Minimum     string `json:"minimum"`
		// 	Recommended string `json:"recommended"`
		// } `json:"mac_requirements"`
		// LinuxRequirements struct {
		// 	Minimum     string `json:"minimum"`
		// 	Recommended string `json:"recommended"`
		// } `json:"linux_requirements"`
		LegalNotice          string   `json:"legal_notice"`
		ExtUserAccountNotice string   `json:"ext_user_account_notice"`
		DRMNotice            string   `json:"drm_notice"`
		Developers           []string `json:"developers"`
		Publishers           []string `json:"publishers"`
		Demos                []struct {
			AppID       int    `json:"appid"`
			Description string `json:"description"`
		} `json:"demos"`
		PriceOverview *struct {
			Currency         string      `json:"currency"`
			Initial          int         `json:"initial"`
			Final            int         `json:"final"`
			DiscountPercent  int         `json:"discount_percent"`
			InitialFormatted string      `json:"initial_formatted"`
			FinalFormatted   string      `json:"final_formatted"`
			RecurringSub     interface{} `json:"recurring_sub"` // Either "false" or a sub id int
			RecurringSubDesc string      `json:"recurring_sub_desc"`
		} `json:"price_overview"`
		// Packages      []int `json:"packages"`
		// PackageGroups []struct {
		// 	Name                    string `json:"name"`
		// 	Title                   string `json:"title"`
		// 	Description             string `json:"description"`
		// 	SelectionText           string `json:"selection_text"`
		// 	SaveText                string `json:"save_text"`
		// 	DisplayType             string `json:"display_type"`
		// 	IsRecurringSubscription bool   `json:"is_recurring_subscription"`
		// 	Subs                    []struct {
		// 		//nolint:misspell // packageid is a Steam API term.
		// 		PackageID                int    `json:"packageid"`
		// 		PercentSavingsText       string `json:"percent_savings_text"`
		// 		PercentSavings           int    `json:"percent_savings"`
		// 		OptionText               string `json:"option_text"`
		// 		OptionDescription        string `json:"option_description"`
		// 		CanGetFreeLicense        bool   `json:"can_get_free_license"`
		// 		IsFreeLicense            bool   `json:"is_free_license"`
		// 		PriceInCentsWithDiscount int    `json:"price_in_cents_with_discount"`
		// 	} `json:"subs"`
		// } `json:"package_groups"`
		Platforms struct {
			Windows bool `json:"windows"`
			Mac     bool `json:"mac"`
			Linux   bool `json:"linux"`
		} `json:"platforms"`
		// Metacritic struct {
		// 	Score int8   `json:"score"`
		// 	URL   string `json:"url"`
		// } `json:"metacritic"`
		Categories []Category `json:"categories"`
		Genres     []Genre    `json:"genres"`
		// Screenshots []struct {
		// 	ID            int    `json:"id"`
		// 	PathThumbnail string `json:"path_thumbnail"`
		// 	PathFull      string `json:"path_full"`
		// } `json:"screenshots"`
		// Movies []struct {
		// 	ID        int    `json:"id"`
		// 	Name      string `json:"name"`
		// 	Thumbnail string `json:"thumbnail"`
		// 	Webm      struct {
		// 		Num480 string `json:"480"`
		// 		Max    string `json:"max"`
		// 	} `json:"webm"`
		// 	Highlight bool `json:"highlight"`
		// } `json:"movies"`
		// Recommendations struct {
		// 	Total int `json:"total"`
		// } `json:"recommendations"`
		// Achievements struct {
		// 	Total       int `json:"total"`
		// 	Highlighted []struct {
		// 		Name string `json:"name"`
		// 		Path string `json:"path"`
		// 	} `json:"highlighted"`
		// } `json:"achievements"`
		ReleaseDate struct {
			ComingSoon bool   `json:"coming_soon"`
			Date       string `json:"date"`
		} `json:"release_date"`
		// SupportInfo struct {
		// 	URL   string `json:"url"`
		// 	Email string `json:"email"`
		// } `json:"support_info"`
		// Background         string `json:"background"`
		// ContentDescriptors struct {
		// 	IDs   interface{}
		// 	Notes interface{}
		// } `json:"content_descriptors"`
	} `json:"data"`
}

// type AppDetail struct {
// 	Success bool `json:"success"`
// 	Data    struct {
// 		Name                string `json:"name"`
// 		SteamAppID          int    `json:"steam_appid"`
// 		DetailedDescription string `json:"detailed_description"`
// 		AboutTheGame        string `json:"about_the_game"`
// 		ShortDescription    string `json:"short_description"`
// 		HeaderImage         string `json:"header_image"`
// 		Website             string `json:"website"`
// 		PriceOverview       struct {
// 			Currency        string `json:"currency"`
// 			Initial         int    `json:"initial"`
// 			Final           int    `json:"final"`
// 			DiscountPercent int    `json:"discount_percent"`
// 			FinalFormatted  string `json:"final_formatted"`
// 		} `json:"price_overview"`
// 		Platforms struct {
// 			Windows bool `json:"windows"`
// 			Mac     bool `json:"mac"`
// 			Linux   bool `json:"linux"`
// 		} `json:"platforms"`
// 		Categories  []Category `json:"categories"`
// 		Genres      []Genre    `json:"genres"`
// 		ReleaseDate struct {
// 			ComingSoon bool   `json:"coming_soon"`
// 			Date       string `json:"date"`
// 		} `json:"release_date"`
// 	} `json:"data"`
// }

type Category struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
}

type Genre struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}
