package api

import (
	"database/sql"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
)

type Store struct {
	queries     *dbgen.Queries
	now         func() time.Time
	steamApiKey string
	steamClient *steamapi.Client
}

func NewStore(database *sql.DB, steamApiKey string, steamClient *steamapi.Client) *Store {
	return &Store{queries: dbgen.New(database), now: time.Now, steamApiKey: steamApiKey, steamClient: steamClient}
}
