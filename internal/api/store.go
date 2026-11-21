package api

import (
	"context"
	"database/sql"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
)

// SteamUserClient resolves a Steam vanity URL to a Steam ID and fetches the
// profile fields stored on steam_users. It's satisfied by *steamapi.Client.
type SteamUserClient interface {
	ResolveVanityURL(ctx context.Context, vanityUrl string) (*steamapi.VanityUrl, error)
	GetPlayerSummaries(ctx context.Context, steamIDs []string) (*steamapi.GetPlayerSummariesResponse, error)
}

type Store struct {
	queries     *dbgen.Queries
	now         func() time.Time
	steamApiKey string
	steamClient SteamUserClient
}

func NewStore(database *sql.DB, steamApiKey string, steamClient SteamUserClient) *Store {
	return &Store{queries: dbgen.New(database), now: time.Now, steamApiKey: steamApiKey, steamClient: steamClient}
}
