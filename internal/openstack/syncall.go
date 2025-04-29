// openstack/syncall.go
package openstack

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/marcdicarlo/osc/internal/config"
	"github.com/marcdicarlo/osc/internal/db"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

// Sync pulls data from OpenStack and populates SQLite.
func SyncAll(sqlDB *sql.DB, cfg *config.Config) error {
	log.Printf("Starting OpenStack sync with compute service: %s, identity service: %s", cfg.OpenStack.ComputeService, cfg.OpenStack.IdentityService)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Drop existing tables
	log.Printf("Dropping existing tables: %s, %s", cfg.Tables.Servers, cfg.Tables.Projects)
	if _, err := sqlDB.ExecContext(ctx, "DROP TABLE IF EXISTS "+cfg.Tables.Servers); err != nil {
		return fmt.Errorf("failed to drop servers table: %w", err)
	}
	if _, err := sqlDB.ExecContext(ctx, "DROP TABLE IF EXISTS "+cfg.Tables.Projects); err != nil {
		return fmt.Errorf("failed to drop projects table: %w", err)
	}

	// Recreate schema
	log.Println("Recreating database schema")
	if err := db.MigrateSchema(ctx, sqlDB, cfg); err != nil {
		return fmt.Errorf("failed to migrate schema: %w", err)
	}

	// Initialize OpenStack clients
	log.Println("Initializing OpenStack clients")
	opts := new(clientconfig.ClientOpts)
	computeClient, err := clientconfig.NewServiceClient(cfg.OpenStack.ComputeService, opts)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}
	identityClient, err := clientconfig.NewServiceClient(cfg.OpenStack.IdentityService, opts)
	if err != nil {
		return fmt.Errorf("failed to create identity client: %w", err)
	}

	// Fetch servers
	log.Printf("Fetching servers (AllTenants: %v)", cfg.OpenStack.AllTenants)
	srvPager, err := servers.List(computeClient, servers.ListOpts{AllTenants: cfg.OpenStack.AllTenants}).AllPages()
	if err != nil {
		return fmt.Errorf("failed to list servers: %w", err)
	}
	srvList, err := servers.ExtractServers(srvPager)
	if err != nil {
		return fmt.Errorf("failed to extract servers: %w", err)
	}
	log.Printf("Found %d servers", len(srvList))

	// Fetch projects
	log.Println("Fetching projects")
	prjPager, err := projects.List(identityClient, nil).AllPages()
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}
	prjList, err := projects.ExtractProjects(prjPager)
	if err != nil {
		return fmt.Errorf("failed to extract projects: %w", err)
	}
	log.Printf("Found %d projects", len(prjList))

	// Insert in transaction
	log.Println("Starting database transaction")
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmtPrj, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.Projects+"(project_id, project_name) VALUES(?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare projects statement: %w", err)
	}
	defer stmtPrj.Close()

	stmtSrv, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.Servers+"(server_id, server_name, project_id) VALUES(?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare servers statement: %w", err)
	}
	defer stmtSrv.Close()

	log.Println("Inserting projects")
	for _, p := range prjList {
		if _, err := stmtPrj.ExecContext(ctx, p.ID, p.Name); err != nil {
			return fmt.Errorf("failed to insert project %s (%s): %w", p.Name, p.ID, err)
		}
	}

	log.Println("Inserting servers")
	for _, s := range srvList {
		if _, err := stmtSrv.ExecContext(ctx, s.ID, s.Name, s.TenantID); err != nil {
			return fmt.Errorf("failed to insert server %s (%s): %w", s.Name, s.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	log.Println("Sync completed successfully")
	return nil
}
