package api

import (
	"strconv"
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

type appSummaryView struct {
	AppID       int32   `json:"appId"`
	Name        *string `json:"name"`
	HeaderImage *string `json:"headerImage"`
}

func newAppSummaryView(app dbgen.App) appSummaryView {
	view := appSummaryView{AppID: app.SteamAppid}
	if app.Name.Valid {
		view.Name = &app.Name.String
	}
	if app.HeaderImage.Valid {
		view.HeaderImage = &app.HeaderImage.String
	}
	return view
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
	UpdatedAt            time.Time  `json:"updatedAt"`
}

func newSteamUserSummaryView(row dbgen.SearchSteamUsersRow) steamUserSummaryView {
	view := steamUserSummaryView{
		SteamID:          formatSteamID(row.SteamID),
		Name:             row.Name,
		WishlistPrivate:  row.WishlistPrivate,
		OwnedAppCount:    row.OwnedAppCount,
		WishlistAppCount: row.WishlistAppCount,
		HasActiveJob:     row.HasActiveJob,
		UpdatedAt:        row.UpdatedAt,
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
