package api

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
)

type steamUserView struct {
	SteamID         string    `json:"steamId"`
	Name            string    `json:"name"`
	WishlistPrivate bool      `json:"wishlistPrivate"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func newSteamUserView(user dbgen.SteamUser) steamUserView {
	return steamUserView{
		SteamID:         formatSteamID(user.SteamID),
		Name:            user.Name,
		WishlistPrivate: user.WishlistPrivate,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}
}

type appDetailView struct {
	ShortDescription     *string `json:"shortDescription"`
	Website              *string `json:"website"`
	PriceCurrency        *string `json:"priceCurrency"`
	PriceInitial         *int32  `json:"priceInitial"`
	PriceFinal           *int32  `json:"priceFinal"`
	PriceDiscountPercent *int32  `json:"priceDiscountPercent"`
	PriceFinalFormatted  *string `json:"priceFinalFormatted"`
	PlatformWindows      *bool   `json:"platformWindows"`
	PlatformMac          *bool   `json:"platformMac"`
	PlatformLinux        *bool   `json:"platformLinux"`
	ReleaseDate          *string `json:"releaseDate"`
	ReleaseComingSoon    *bool   `json:"releaseComingSoon"`
}

func newAppDetailView(app dbgen.App) appDetailView {
	return appDetailView{
		ShortDescription:     nullableString(app.ShortDescription),
		Website:              nullableString(app.Website),
		PriceCurrency:        nullableString(app.PriceCurrency),
		PriceInitial:         nullableInt32(app.PriceInitial),
		PriceFinal:           nullableInt32(app.PriceFinal),
		PriceDiscountPercent: nullableInt32(app.PriceDiscountPercent),
		PriceFinalFormatted:  nullableString(app.PriceFinalFormatted),
		PlatformWindows:      nullableBool(app.PlatformWindows),
		PlatformMac:          nullableBool(app.PlatformMac),
		PlatformLinux:        nullableBool(app.PlatformLinux),
		ReleaseDate:          nullableString(app.ReleaseDate),
		ReleaseComingSoon:    nullableBool(app.ReleaseComingSoon),
	}
}

type appSummaryView struct {
	AppID       int32   `json:"appId"`
	Name        *string `json:"name"`
	HeaderImage *string `json:"headerImage"`
	appDetailView
}

func newAppSummaryView(app dbgen.App) appSummaryView {
	return appSummaryView{
		AppID:         app.SteamAppid,
		Name:          nullableString(app.Name),
		HeaderImage:   nullableString(app.HeaderImage),
		appDetailView: newAppDetailView(app),
	}
}

type commonGameView struct {
	AppID       int32   `json:"appId"`
	Name        *string `json:"name"`
	HeaderImage *string `json:"headerImage"`
	appDetailView
	OwnedCount         int64    `json:"ownedCount"`
	WishlistCount      int64    `json:"wishlistCount"`
	TotalCommonUsers   int64    `json:"totalCommonUsers"`
	OwnedSteamIDs      []string `json:"ownedSteamIds"`
	WishlistedSteamIDs []string `json:"wishlistedSteamIds"`
}

func newCommonGameView(row dbgen.ListCommonGamesRow) commonGameView {
	return commonGameView{
		AppID:       row.SteamAppid,
		Name:        nullableString(row.Name),
		HeaderImage: nullableString(row.HeaderImage),
		appDetailView: appDetailView{
			ShortDescription:     nullableString(row.ShortDescription),
			Website:              nullableString(row.Website),
			PriceCurrency:        nullableString(row.PriceCurrency),
			PriceInitial:         nullableInt32(row.PriceInitial),
			PriceFinal:           nullableInt32(row.PriceFinal),
			PriceDiscountPercent: nullableInt32(row.PriceDiscountPercent),
			PriceFinalFormatted:  nullableString(row.PriceFinalFormatted),
			PlatformWindows:      nullableBool(row.PlatformWindows),
			PlatformMac:          nullableBool(row.PlatformMac),
			PlatformLinux:        nullableBool(row.PlatformLinux),
			ReleaseDate:          nullableString(row.ReleaseDate),
			ReleaseComingSoon:    nullableBool(row.ReleaseComingSoon),
		},
		OwnedCount:         row.OwnedCount,
		WishlistCount:      row.WishlistCount,
		TotalCommonUsers:   row.TotalCommonUsers,
		OwnedSteamIDs:      splitSteamIDList(row.OwnedSteamIds),
		WishlistedSteamIDs: splitSteamIDList(row.WishlistedSteamIds),
	}
}

type steamUserSummaryView struct {
	SteamID              string     `json:"steamId"`
	Name                 string     `json:"name"`
	WishlistPrivate      bool       `json:"wishlistPrivate"`
	OwnedAppCount        int64      `json:"ownedAppCount"`
	WishlistAppCount     int64      `json:"wishlistAppCount"`
	LatestImportStatus   *string    `json:"latestImportStatus"`
	LatestImportFinished *time.Time `json:"latestImportFinishedAt"`
	HasActiveJob         bool       `json:"hasActiveJob"`
	HasSucceededImport   bool       `json:"hasSucceededImport"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

func newSteamUserSummaryView(row dbgen.SearchSteamUsersRow) steamUserSummaryView {
	view := steamUserSummaryView{
		SteamID:            formatSteamID(row.SteamID),
		Name:               row.Name,
		WishlistPrivate:    row.WishlistPrivate,
		OwnedAppCount:      row.OwnedAppCount,
		WishlistAppCount:   row.WishlistAppCount,
		HasActiveJob:       row.HasActiveJob,
		HasSucceededImport: row.HasSucceededImport,
		UpdatedAt:          row.UpdatedAt,
	}
	if row.LatestImportStatus.Valid {
		view.LatestImportStatus = &row.LatestImportStatus.String
	}
	if row.LatestImportFinishedAt.Valid {
		view.LatestImportFinished = &row.LatestImportFinishedAt.Time
	}
	return view
}

type importJobView struct {
	ID                    int64      `json:"id"`
	SteamID               string     `json:"steamId"`
	Status                string     `json:"status"`
	TotalApps             int32      `json:"totalApps"`
	ProcessedApps         int32      `json:"processedApps"`
	SkippedApps           int32      `json:"skippedApps"`
	FailedApps            int32      `json:"failedApps"`
	WishlistTotalApps     int32      `json:"wishlistTotalApps"`
	WishlistProcessedApps int32      `json:"wishlistProcessedApps"`
	WishlistSkippedApps   int32      `json:"wishlistSkippedApps"`
	WishlistFailedApps    int32      `json:"wishlistFailedApps"`
	Error                 *string    `json:"error"`
	CreatedAt             time.Time  `json:"createdAt"`
	StartedAt             *time.Time `json:"startedAt"`
	FinishedAt            *time.Time `json:"finishedAt"`
}

func newImportJobView(job dbgen.ImportJob) importJobView {
	view := importJobView{
		ID:                    job.ID,
		SteamID:               formatSteamID(job.SteamID),
		Status:                job.Status,
		TotalApps:             job.TotalApps,
		ProcessedApps:         job.ProcessedApps,
		SkippedApps:           job.SkippedApps,
		FailedApps:            job.FailedApps,
		WishlistTotalApps:     job.WishlistTotalApps,
		WishlistProcessedApps: job.WishlistProcessedApps,
		WishlistSkippedApps:   job.WishlistSkippedApps,
		WishlistFailedApps:    job.WishlistFailedApps,
		CreatedAt:             job.CreatedAt,
	}
	if job.Error.Valid {
		view.Error = &job.Error.String
	}
	if job.StartedAt.Valid {
		view.StartedAt = &job.StartedAt.Time
	}
	if job.FinishedAt.Valid {
		view.FinishedAt = &job.FinishedAt.Time
	}
	return view
}

func formatSteamID(steamID int64) string {
	return strconv.FormatInt(steamID, 10)
}

func splitSteamIDList(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullableInt32(value sql.NullInt32) *int32 {
	if !value.Valid {
		return nil
	}
	return &value.Int32
}

func nullableBool(value sql.NullBool) *bool {
	if !value.Valid {
		return nil
	}
	return &value.Bool
}
