// db/db.go
package db

import (
	"context"
	"database/sql"

	"github.com/marcdicarlo/osc/internal/config"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB opens SQLite, sets pragmas, and migrates schema based on config.
func InitDB(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", cfg.DBFile)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	// enable foreign keys
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, err
	}
	// migrate schema
	if err := MigrateSchema(ctx, db, cfg); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// MigrateSchema creates tables for projects and servers.
func MigrateSchema(ctx context.Context, db *sql.DB, cfg *config.Config) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ` + cfg.Tables.Projects + ` (
			project_id   TEXT PRIMARY KEY,
			project_name TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS ` + cfg.Tables.Servers + ` (
			server_id   TEXT PRIMARY KEY,
			server_name TEXT NOT NULL,
			project_id  TEXT NOT NULL,
			FOREIGN KEY(project_id) REFERENCES ` + cfg.Tables.Projects + `(project_id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS ` + cfg.Tables.SecGrps + ` (
			secgrp_id   TEXT PRIMARY KEY,
			secgrp_name TEXT NOT NULL,
			project_id  TEXT NOT NULL,
			FOREIGN KEY(project_id) REFERENCES ` + cfg.Tables.Projects + `(project_id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS ` + cfg.Tables.SecGrpRules + ` (
			rule_id          TEXT PRIMARY KEY,
			secgrp_id       TEXT NOT NULL,
			direction       TEXT NOT NULL,
			ethertype       TEXT NOT NULL,
			protocol        TEXT,
			port_range_min  INTEGER,
			port_range_max  INTEGER,
			remote_ip_prefix TEXT,
			FOREIGN KEY(secgrp_id) REFERENCES ` + cfg.Tables.SecGrps + `(secgrp_id) ON DELETE CASCADE
		)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}
