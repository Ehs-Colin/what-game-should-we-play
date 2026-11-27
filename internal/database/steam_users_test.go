package database_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbtest"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	customSteamIDA = int64(76561198000000301)
	customSteamIDB = int64(76561198000000302)
	vanitySteamID  = int64(76561198000000303)
	twinSteamID    = int64(76561198000000304)
)

func upsertUser(t *testing.T, queries *dbgen.Queries, steamID int64, name string, custom bool) dbgen.SteamUser {
	t.Helper()
	user, err := queries.UpsertSteamUser(context.Background(), dbgen.UpsertSteamUserParams{
		SteamID:        steamID,
		Name:           name,
		CustomUsername: custom,
	})
	if err != nil {
		t.Fatalf("upsert user %d name %q: %v", steamID, name, err)
	}
	return user
}

func newQueries(t *testing.T) *dbgen.Queries {
	t.Helper()
	return dbgen.New(dbtest.Open(t))
}

func TestUpsertSteamUserCustomUsernameRoundTrip(t *testing.T) {
	queries := newQueries(t)

	stored := upsertUser(t, queries, customSteamIDA, "RaidBuddy", true)
	if !stored.CustomUsername {
		t.Errorf("upsert returned custom_username = false, want true")
	}
	fetched, err := queries.GetSteamUser(context.Background(), customSteamIDA)
	if err != nil {
		t.Fatalf("get steam user: %v", err)
	}
	if !fetched.CustomUsername {
		t.Errorf("stored custom_username = false, want true")
	}

	// A later vanity import overwrites the label and clears the flag.
	stored = upsertUser(t, queries, customSteamIDA, "someplayer", false)
	if stored.CustomUsername {
		t.Errorf("vanity re-upsert kept custom_username = true")
	}
	if stored.Name != "someplayer" {
		t.Errorf("name = %q, want the vanity", stored.Name)
	}
}

func TestCustomNameUniquenessIndex(t *testing.T) {
	queries := newQueries(t)
	upsertUser(t, queries, customSteamIDA, "Shared Label", true)

	// The same case-insensitive name as another custom row is rejected.
	_, err := queries.UpsertSteamUser(context.Background(), dbgen.UpsertSteamUserParams{
		SteamID:        customSteamIDB,
		Name:           "shared label",
		CustomUsername: true,
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("duplicate custom name error = %v, want a 23505 unique violation", err)
	}

	// Vanity rows (flag false) ignore the index entirely, including one that
	// shadows an existing custom name, and duplicate vanity names as before.
	upsertUser(t, queries, vanitySteamID, "SHARED LABEL", false)
	upsertUser(t, queries, twinSteamID, "Twin", false)
	upsertUser(t, queries, customSteamIDB, "twin", false)

	// Updating the same row's own custom name stays allowed.
	upsertUser(t, queries, customSteamIDA, "Renamed Label", true)
}

func TestGetSteamUserByCustomName(t *testing.T) {
	queries := newQueries(t)
	upsertUser(t, queries, customSteamIDA, "RaidBuddy", true)
	upsertUser(t, queries, vanitySteamID, "raidbuddy", false)

	// The match is case-insensitive and only sees custom-named rows.
	user, err := queries.GetSteamUserByCustomName(context.Background(), "RAIDBUDDY")
	if err != nil {
		t.Fatalf("get by custom name: %v", err)
	}
	if user.SteamID != customSteamIDA {
		t.Errorf("steam_id = %d, want the custom row %d", user.SteamID, customSteamIDA)
	}

	_, err = queries.GetSteamUserByCustomName(context.Background(), "nobody")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown name error = %v, want sql.ErrNoRows", err)
	}
}
