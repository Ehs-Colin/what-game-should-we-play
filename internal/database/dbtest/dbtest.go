// Package dbtest gives tests an isolated, migrated Postgres schema.
package dbtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database"
)

// EnvVar names the Postgres URL used by tests. Tests that need a database are
// skipped when it's unset.
const EnvVar = "TEST_DATABASE_URL"

// Open creates a fresh schema, runs the migrations in it, and drops it when
// the test finishes. Tests that use it can run in parallel.
func Open(t testing.TB) *sql.DB {
	t.Helper()
	baseURL := os.Getenv(EnvVar)
	if baseURL == "" {
		t.Skipf("%s is not set; skipping database test", EnvVar)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := database.Open(ctx, baseURL, 1)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	t.Cleanup(func() { admin.Close() })

	schema := "test_" + randomSuffix(t)
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop schema %s: %v", schema, err)
		}
	})

	schemaURL, err := withSearchPath(baseURL, schema)
	if err != nil {
		t.Fatalf("build schema URL: %v", err)
	}
	db, err := database.Open(ctx, schemaURL, 5)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	// Registered after the DROP SCHEMA cleanup, so it runs first.
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return db
}

// withSearchPath adds search_path to the URL. pgx passes unknown URL
// parameters to the server as runtime parameters.
func withSearchPath(rawURL, schema string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func randomSuffix(t testing.TB) string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("random schema name: %v", err)
	}
	return hex.EncodeToString(buf)
}
