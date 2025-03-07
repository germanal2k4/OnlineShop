package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

func NewDB(dsn, migrationsDir string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db error: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db error: %w", err)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return nil, fmt.Errorf("goose up error: %w", err)
	}

	return db, nil
}
