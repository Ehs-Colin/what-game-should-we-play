package api

import (
	"context"
	"database/sql"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
)

// VanityResolver resolves a Steam vanity URL to a Steam ID. It's satisfied by
// *steamapi.Client.
type VanityResolver interface {
	ResolveVanityURL(ctx context.Context, vanityUrl string) (*steamapi.VanityUrl, error)
}

type Store struct {
	queries     *dbgen.Queries
	now         func() time.Time
	steamApiKey string
	steamClient VanityResolver
}

func NewStore(database *sql.DB, steamApiKey string, steamClient VanityResolver) *Store {
	return &Store{queries: dbgen.New(database), now: time.Now, steamApiKey: steamApiKey, steamClient: steamClient}
}
