// openstack/syncall.go
package openstack

import (
	"context"
	"database/sql"

	"github.com/marcdicarlo/osc/internal/config"
	dbpkg "github.com/marcdicarlo/osc/internal/db"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

// Sync pulls data from OpenStack and populates SQLite.
func SyncAll(db *sql.DB, cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Drop existing tables
	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+cfg.Tables.Servers); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+cfg.Tables.Projects); err != nil {
		return err
	}

	// Recreate schema
	if err := dbpkg.MigrateSchema(ctx, db, cfg); err != nil {
		return err
	}

	// Initialize OpenStack clients
	opts := new(clientconfig.ClientOpts)
	computeClient, err := clientconfig.NewServiceClient(cfg.OpenStack.ComputeService, opts)
	if err != nil {
		return err
	}
	identityClient, err := clientconfig.NewServiceClient(cfg.OpenStack.IdentityService, opts)
	if err != nil {
		return err
	}

	// Fetch servers
	srvPager, err := servers.List(computeClient, servers.ListOpts{AllTenants: cfg.OpenStack.AllTenants}).AllPages()
	if err != nil {
		return err
	}
	srvList, err := servers.ExtractServers(srvPager)
	if err != nil {
		return err
	}

	// Fetch projects
	prjPager, err := projects.List(identityClient, nil).AllPages()
	if err != nil {
		return err
	}
	prjList, err := projects.ExtractProjects(prjPager)
	if err != nil {
		return err
	}

	// Insert in transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmtPrj, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.Projects+"(project_id, project_name) VALUES(?, ?)")
	if err != nil {
		return err
	}
	defer stmtPrj.Close()

	stmtSrv, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.Servers+"(server_id, server_name, project_id) VALUES(?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmtSrv.Close()

	for _, p := range prjList {
		if _, err := stmtPrj.ExecContext(ctx, p.ID, p.Name); err != nil {
			return err
		}
	}
	for _, s := range srvList {
		if _, err := stmtSrv.ExecContext(ctx, s.ID, s.Name, s.TenantID); err != nil {
			return err
		}
	}

	return tx.Commit()
}
