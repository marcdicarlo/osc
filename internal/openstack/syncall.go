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
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
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

	// Fetch security groups for all projects
	var allSecurityGroups []struct {
		ProjectID string
		Group     groups.SecGroup
	}

	log.Println("Fetching security groups for all projects")
	for _, p := range prjList {
		sgPager, err := groups.List(computeClient, groups.ListOpts{TenantID: p.ID}).AllPages()
		if err != nil {
			return fmt.Errorf("failed to list security groups for project %s: %w", p.ID, err)
		}
		sgList, err := groups.ExtractGroups(sgPager)
		if err != nil {
			return fmt.Errorf("failed to extract security groups for project %s: %w", p.ID, err)
		}
		log.Printf("Found %d security groups for project %s", len(sgList), p.ID)

		for _, sg := range sgList {
			allSecurityGroups = append(allSecurityGroups, struct {
				ProjectID string
				Group     groups.SecGroup
			}{
				ProjectID: p.ID,
				Group:     sg,
			})
		}
	}
	log.Printf("Total security groups found: %d", len(allSecurityGroups))

	// Insert in transaction
	log.Println("Starting database transaction")

	// Check if database is still responsive
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database not responsive before transaction: %w", err)
	}
	log.Println("Database connection confirmed")

	// Verify foreign keys are enabled
	var fkEnabled int
	if err := sqlDB.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		return fmt.Errorf("failed to check foreign_keys pragma: %w", err)
	}
	log.Printf("Foreign keys enabled: %v", fkEnabled == 1)

	tx, err := sqlDB.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction (timeout: %v): %w", cfg.DBTimeout, err)
	}
	log.Println("Transaction successfully started")
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Warning: failed to rollback transaction: %v", err)
		}
	}()

	log.Println("Preparing project statement")
	stmtPrj, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.Projects+"(project_id, project_name) VALUES(?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare projects statement: %w", err)
	}
	defer stmtPrj.Close()

	log.Println("Preparing server statement")
	stmtSrv, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.Servers+"(server_id, server_name, project_id) VALUES(?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare servers statement: %w", err)
	}
	defer stmtSrv.Close()

	log.Println("Preparing security group statement")
	stmtSG, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.SecGrps+"(secgrp_id, secgrp_name, project_id) VALUES(?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare security groups statement: %w", err)
	}
	defer stmtSG.Close()

	log.Println("Preparing security group rules statement")
	stmtSGRule, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.SecGrpRules+"(rule_id, secgrp_id, direction, ethertype, protocol, port_range_min, port_range_max, remote_ip_prefix) VALUES(?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare security group rules statement: %w", err)
	}
	defer stmtSGRule.Close()

	log.Printf("Starting to insert %d projects", len(prjList))
	for i, p := range prjList {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled during project insertion: %w", err)
		}
		if _, err := stmtPrj.ExecContext(ctx, p.ID, p.Name); err != nil {
			return fmt.Errorf("failed to insert project %s (%s) at index %d: %w", p.Name, p.ID, i, err)
		}
		if (i+1)%100 == 0 {
			log.Printf("Inserted %d/%d projects", i+1, len(prjList))
		}
	}

	log.Printf("Starting to insert %d servers", len(srvList))
	for i, s := range srvList {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled during server insertion: %w", err)
		}
		if _, err := stmtSrv.ExecContext(ctx, s.ID, s.Name, s.TenantID); err != nil {
			return fmt.Errorf("failed to insert server %s (%s) at index %d: %w", s.Name, s.ID, i, err)
		}
		if (i+1)%100 == 0 {
			log.Printf("Inserted %d/%d servers", i+1, len(srvList))
		}
	}

	log.Printf("Starting to insert %d security groups and their rules", len(allSecurityGroups))
	for i, sg := range allSecurityGroups {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled during security group insertion: %w", err)
		}

		if _, err := stmtSG.ExecContext(ctx, sg.Group.ID, sg.Group.Name, sg.ProjectID); err != nil {
			return fmt.Errorf("failed to insert security group %s (%s) at index %d: %w", sg.Group.Name, sg.Group.ID, i, err)
		}

		for j, rule := range sg.Group.Rules {
			if _, err := stmtSGRule.ExecContext(ctx,
				rule.ID,
				sg.Group.ID,
				rule.Direction,
				rule.EtherType,
				rule.Protocol,
				rule.PortRangeMin,
				rule.PortRangeMax,
				rule.RemoteIPPrefix); err != nil {
				return fmt.Errorf("failed to insert rule %s for security group %s at index %d: %w", rule.ID, sg.Group.ID, j, err)
			}
		}

		if (i+1)%10 == 0 {
			log.Printf("Inserted %d/%d security groups", i+1, len(allSecurityGroups))
		}
	}

	log.Println("Committing transaction")
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	log.Println("Sync completed successfully")
	return nil
}
