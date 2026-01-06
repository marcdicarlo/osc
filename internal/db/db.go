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
			ipv4_addr   TEXT,
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
			remote_group_id TEXT,
			FOREIGN KEY(secgrp_id) REFERENCES ` + cfg.Tables.SecGrps + `(secgrp_id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS ` + cfg.Tables.Volumes + ` (
			volume_id    TEXT PRIMARY KEY,
			volume_name  TEXT NOT NULL,
			size_gb      INTEGER NOT NULL,
			volume_type  TEXT,
			project_id   TEXT,
			FOREIGN KEY(project_id) REFERENCES ` + cfg.Tables.Projects + `(project_id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS ` + cfg.Tables.ServerSecGrps + ` (
			server_id TEXT NOT NULL,
			secgrp_id TEXT NOT NULL,
			PRIMARY KEY (server_id, secgrp_id),
			FOREIGN KEY(server_id) REFERENCES ` + cfg.Tables.Servers + `(server_id) ON DELETE CASCADE,
			FOREIGN KEY(secgrp_id) REFERENCES ` + cfg.Tables.SecGrps + `(secgrp_id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS ` + cfg.Tables.ServerVolumes + ` (
			server_id   TEXT NOT NULL,
			volume_id   TEXT NOT NULL,
			device_path TEXT NOT NULL,
			PRIMARY KEY (server_id, volume_id),
			FOREIGN KEY(server_id) REFERENCES ` + cfg.Tables.Servers + `(server_id) ON DELETE CASCADE,
			FOREIGN KEY(volume_id) REFERENCES ` + cfg.Tables.Volumes + `(volume_id) ON DELETE CASCADE
		)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return err
		}
	}

	// Migration: Add remote_group_id column if it doesn't exist (for existing databases)
	if err := addColumnIfNotExists(ctx, db, cfg.Tables.SecGrpRules, "remote_group_id", "TEXT"); err != nil {
		return err
	}

	return nil
}

// addColumnIfNotExists adds a column to a table if it doesn't already exist.
func addColumnIfNotExists(ctx context.Context, db *sql.DB, tableName, columnName, columnType string) error {
	// Check if column exists by querying table_info
	var exists bool
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+tableName+")")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == columnName {
			exists = true
			break
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Add column if it doesn't exist
	if !exists {
		_, err := db.ExecContext(ctx, "ALTER TABLE "+tableName+" ADD COLUMN "+columnName+" "+columnType)
		return err
	}

	return nil
}
