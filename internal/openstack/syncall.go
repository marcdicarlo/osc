// openstack/syncall.go
package openstack

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/marcdicarlo/osc/internal/config"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/utils/openstack/clientconfig"
	"golang.org/x/sync/semaphore"
)

// initOpenStackClients initializes and verifies connectivity to all required OpenStack services
func initOpenStackClients(cfg *config.Config) (*gophercloud.ServiceClient, *gophercloud.ServiceClient, *gophercloud.ServiceClient, error) {
	opts := new(clientconfig.ClientOpts)

	// Initialize compute client
	computeClient, err := clientconfig.NewServiceClient(cfg.OpenStack.ComputeService, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create compute client: %w", err)
	}

	// Verify compute connectivity with a simple list operation
	// if _, err := servers.List(computeClient, servers.ListOpts{Limit: 1}).AllPages(); err != nil {
	// 	return nil, nil, nil, fmt.Errorf("failed to verify compute service connectivity: %w", err)
	// }

	// Initialize identity client
	identityClient, err := clientconfig.NewServiceClient(cfg.OpenStack.IdentityService, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create identity client: %w", err)
	}

	// Verify identity connectivity
	// if _, err := projects.List(identityClient, nil).AllPages(); err != nil {
	// 	return nil, nil, nil, fmt.Errorf("failed to verify identity service connectivity: %w", err)
	// }

	// Initialize network client
	networkClient, err := clientconfig.NewServiceClient("network", opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create network client: %w", err)
	}

	// Verify network connectivity
	// if _, err := groups.List(networkClient, groups.ListOpts{Limit: 1}).AllPages(); err != nil {
	// 	return nil, nil, nil, fmt.Errorf("failed to verify network service connectivity: %w", err)
	// }

	return computeClient, identityClient, networkClient, nil
}

// clearTables safely clears all tables while maintaining their structure
func clearTables(ctx context.Context, tx *sql.Tx, cfg *config.Config) error {
	tables := []string{
		cfg.Tables.Servers,
		cfg.Tables.Projects,
		cfg.Tables.SecGrps,
		cfg.Tables.SecGrpRules,
	}

	for _, table := range tables {
		log.Printf("Clearing table: %s", table)
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("failed to clear table %s: %w", table, err)
		}
	}
	return nil
}

// securityGroupResult holds the result of fetching security groups for a single project
type securityGroupResult struct {
	ProjectID string
	Groups    []groups.SecGroup
	Error     error
}

// fetchSecurityGroupsParallel fetches security groups for all projects concurrently using a worker pool
func fetchSecurityGroupsParallel(networkClient *gophercloud.ServiceClient, projectList []projects.Project, cfg *config.Config) ([]struct {
	ProjectID string
	Group     groups.SecGroup
}, error) {
	numProjects := len(projectList)
	if numProjects == 0 {
		return nil, nil
	}

	log.Printf("Fetching security groups for %d projects using %d workers", numProjects, cfg.OpenStack.MaxWorkers)

	// Create a semaphore to limit concurrent workers
	sem := semaphore.NewWeighted(int64(cfg.OpenStack.MaxWorkers))

	// Channel to collect results
	resultsChan := make(chan securityGroupResult, numProjects)

	// WaitGroup to track all goroutines
	var wg sync.WaitGroup

	// Launch workers for each project
	startTime := time.Now()
	for _, p := range projectList {
		wg.Add(1)
		go func(project projects.Project) {
			defer wg.Done()

			// Acquire semaphore (blocks if max workers reached)
			ctx, cancel := context.WithTimeout(context.Background(), cfg.OpenStack.WorkerTimeout)
			defer cancel()

			if err := sem.Acquire(ctx, 1); err != nil {
				resultsChan <- securityGroupResult{
					ProjectID: project.ID,
					Error:     fmt.Errorf("failed to acquire semaphore: %w", err),
				}
				return
			}
			defer sem.Release(1)

			// Fetch security groups for this project
			sgPager, err := groups.List(networkClient, groups.ListOpts{TenantID: project.ID}).AllPages()
			if err != nil {
				resultsChan <- securityGroupResult{
					ProjectID: project.ID,
					Error:     fmt.Errorf("failed to list security groups: %w", err),
				}
				return
			}

			sgList, err := groups.ExtractGroups(sgPager)
			if err != nil {
				resultsChan <- securityGroupResult{
					ProjectID: project.ID,
					Error:     fmt.Errorf("failed to extract security groups: %w", err),
				}
				return
			}

			resultsChan <- securityGroupResult{
				ProjectID: project.ID,
				Groups:    sgList,
				Error:     nil,
			}
		}(p)
	}

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var allSecurityGroups []struct {
		ProjectID string
		Group     groups.SecGroup
	}
	totalGroups := 0
	processedProjects := 0

	for result := range resultsChan {
		processedProjects++

		if result.Error != nil {
			return nil, fmt.Errorf("failed to fetch security groups for project %s: %w", result.ProjectID, result.Error)
		}

		// Add all groups from this project to the collection
		for _, sg := range result.Groups {
			allSecurityGroups = append(allSecurityGroups, struct {
				ProjectID string
				Group     groups.SecGroup
			}{
				ProjectID: result.ProjectID,
				Group:     sg,
			})
		}
		totalGroups += len(result.Groups)

		// Log progress every 10 projects
		if processedProjects%10 == 0 {
			log.Printf("Progress: %d/%d projects processed, %d security groups found so far", processedProjects, numProjects, totalGroups)
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("Fetched security groups from %d projects in %v (%d total groups, %.2f projects/sec)",
		numProjects, elapsed, totalGroups, float64(numProjects)/elapsed.Seconds())

	return allSecurityGroups, nil
}

// Sync pulls data from OpenStack and populates SQLite.
func SyncAll(sqlDB *sql.DB, cfg *config.Config) error {
	log.Printf("Starting OpenStack sync with compute service: %s, identity service: %s", cfg.OpenStack.ComputeService, cfg.OpenStack.IdentityService)

	// First verify OpenStack connectivity before making any database changes
	// log.Println("Verifying OpenStack connectivity")
	computeClient, identityClient, networkClient, err := initOpenStackClients(cfg)
	if err != nil {
		return fmt.Errorf("OpenStack authentication failed: %w", err)
	}
	log.Println("Successfully authenticated with OpenStack services")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Start transaction for database operations
	tx, err := sqlDB.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Warning: failed to rollback transaction: %v", err)
		}
	}()

	// Clear existing data
	if err := clearTables(ctx, tx, cfg); err != nil {
		return fmt.Errorf("failed to clear tables: %w", err)
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

	// Fetch security groups for all projects using parallel workers
	allSecurityGroups, err := fetchSecurityGroupsParallel(networkClient, prjList, cfg)
	if err != nil {
		return fmt.Errorf("failed to fetch security groups: %w", err)
	}
	log.Printf("Total security groups found: %d", len(allSecurityGroups))

	// Prepare statements
	log.Println("Preparing statements")
	stmtPrj, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.Projects+"(project_id, project_name) VALUES(?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare projects statement: %w", err)
	}
	defer stmtPrj.Close()

	stmtSrv, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.Servers+"(server_id, server_name, project_id, ipv4_addr) VALUES(?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare servers statement: %w", err)
	}
	defer stmtSrv.Close()

	stmtSG, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.SecGrps+"(secgrp_id, secgrp_name, project_id) VALUES(?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare security groups statement: %w", err)
	}
	defer stmtSG.Close()

	stmtSGRule, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.SecGrpRules+"(rule_id, secgrp_id, direction, ethertype, protocol, port_range_min, port_range_max, remote_ip_prefix) VALUES(?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare security group rules statement: %w", err)
	}
	defer stmtSGRule.Close()

	// Insert data
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

		// Get the first IPv4 address from the server's addresses
		var ipv4Addr string
		for _, addresses := range s.Addresses {
			for _, addr := range addresses.([]interface{}) {
				if address, ok := addr.(map[string]interface{}); ok {
					if address["version"].(float64) == 4 {
						ipv4Addr = address["addr"].(string)
						break
					}
				}
			}
			if ipv4Addr != "" {
				break
			}
		}

		if _, err := stmtSrv.ExecContext(ctx, s.ID, s.Name, s.TenantID, ipv4Addr); err != nil {
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
