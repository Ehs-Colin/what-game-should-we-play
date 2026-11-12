package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

func Open(ctx context.Context, databaseURL string, maxOpenConnections int) (*sql.DB, error) {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	database.SetMaxOpenConns(maxOpenConnections)
	database.SetMaxIdleConns(maxOpenConnections)

	if err := database.PingContext(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return database, nil
}

func Migrate(ctx context.Context, database *sql.DB) error {
	migrationFiles, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, database, migrationFiles)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply database migrations: %w", err)
	}
	return nil
}
