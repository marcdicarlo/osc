// openstack/syncall.go
package openstack

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/marcdicarlo/osc/internal/config"
	"github.com/marcdicarlo/osc/internal/logx"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/gophercloud/utils/openstack/clientconfig"
	"golang.org/x/sync/semaphore"
)

const apiWatchdogInterval = 15 * time.Second

func phaseError(phase string, err error) error {
	return fmt.Errorf("phase=%s: %w", phase, err)
}

func withAPIWatchdog(name string, fn func() error) error {
	stop := logx.StartWatchdog(name, apiWatchdogInterval)
	defer stop()
	return fn()
}

func withAPIWatchdogResult[T any](name string, fn func() (T, error)) (T, error) {
	stop := logx.StartWatchdog(name, apiWatchdogInterval)
	defer stop()
	return fn()
}

func attachHTTPTracing(serviceName string, client *gophercloud.ServiceClient) {
	if !logx.DebugEnabled() || client == nil || client.ProviderClient == nil {
		return
	}

	base := client.ProviderClient.HTTPClient.Transport
	if _, ok := base.(*logx.LoggingRoundTripper); ok {
		return
	}

	client.ProviderClient.HTTPClient.Transport = logx.NewLoggingRoundTripper(base)
	logx.Debugf("http_tracing_enabled service=%s endpoint=%s", serviceName, logx.RedactURL(client.Endpoint))
}

func initServiceClient(serviceName string, opts *clientconfig.ClientOpts) (*gophercloud.ServiceClient, error) {
	step := logx.StepStart("init_service_client", "service", serviceName)
	client, err := clientconfig.NewServiceClient(serviceName, opts)
	if err != nil {
		step.DoneWithError(err, "service", serviceName)
		return nil, phaseError("init_"+serviceName+"_client", err)
	}
	attachHTTPTracing(serviceName, client)
	step.Done("service", serviceName, "endpoint", logx.RedactURL(client.Endpoint))
	return client, nil
}

// initOpenStackClients initializes and verifies connectivity to all required OpenStack services
func initOpenStackClients(cfg *config.Config) (*gophercloud.ServiceClient, *gophercloud.ServiceClient, *gophercloud.ServiceClient, *gophercloud.ServiceClient, error) {
	opts := new(clientconfig.ClientOpts)
	logx.Debugf("openstack_client_init_start compute=%s identity=%s", cfg.OpenStack.ComputeService, cfg.OpenStack.IdentityService)

	// Initialize compute client
	computeClient, err := initServiceClient(cfg.OpenStack.ComputeService, opts)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Initialize identity client
	identityClient, err := initServiceClient(cfg.OpenStack.IdentityService, opts)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Initialize network client
	networkClient, err := initServiceClient("network", opts)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Initialize block storage client
	blockStorageClient, err := initServiceClient("volume", opts)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if logx.DebugEnabled() {
		transport := blockStorageClient.ProviderClient.HTTPClient.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		logx.Debugf("openstack_client_init_done transport=%T", transport)
	}

	return computeClient, identityClient, networkClient, blockStorageClient, nil
}

// clearTables safely clears all tables while maintaining their structure
func clearTables(ctx context.Context, tx *sql.Tx, cfg *config.Config) error {
	// Clear junction tables first due to FK constraints
	tables := []string{
		cfg.Tables.ServerVolumes,
		cfg.Tables.ServerSecGrps,
		cfg.Tables.Volumes,
		cfg.Tables.SecGrpRules,
		cfg.Tables.SecGrps,
		cfg.Tables.Servers,
		cfg.Tables.Projects,
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
			var sgPager pagination.Page
			err := withAPIWatchdog("list_security_groups_project_"+project.ID, func() error {
				var listErr error
				sgPager, listErr = groups.List(networkClient, groups.ListOpts{TenantID: project.ID}).AllPages()
				return listErr
			})
			if err != nil {
				resultsChan <- securityGroupResult{
					ProjectID: project.ID,
					Error:     phaseError("list_security_groups", err),
				}
				return
			}

			sgList, err := groups.ExtractGroups(sgPager)
			if err != nil {
				resultsChan <- securityGroupResult{
					ProjectID: project.ID,
					Error:     phaseError("extract_security_groups", err),
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
	authStep := logx.StepStart("sync_all_auth", "phase", "auth")
	computeClient, identityClient, networkClient, blockStorageClient, err := initOpenStackClients(cfg)
	if err != nil {
		authStep.DoneWithError(err, "phase", "auth")
		return phaseError("auth", err)
	}
	authStep.Done("phase", "auth")
	log.Println("Successfully authenticated with OpenStack services")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Start transaction for database operations
	txStep := logx.StepStart("sync_all_begin_tx", "phase", "begin_transaction")
	tx, err := sqlDB.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
	if err != nil {
		txStep.DoneWithError(err, "phase", "begin_transaction")
		return phaseError("begin_transaction", err)
	}
	txStep.Done("phase", "begin_transaction")
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Warning: failed to rollback transaction: %v", err)
		}
	}()

	// Clear existing data
	clearStep := logx.StepStart("sync_all_clear_tables", "phase", "clear_tables")
	if err := clearTables(ctx, tx, cfg); err != nil {
		clearStep.DoneWithError(err, "phase", "clear_tables")
		return phaseError("clear_tables", err)
	}
	clearStep.Done("phase", "clear_tables")

	// Fetch servers
	log.Printf("Fetching servers (AllTenants: %v)", cfg.OpenStack.AllTenants)
	fetchServersStep := logx.StepStart("sync_all_fetch_servers", "phase", "fetch_servers")
	var srvPager pagination.Page
	err = withAPIWatchdog("list_servers_all", func() error {
		var listErr error
		srvPager, listErr = servers.List(computeClient, servers.ListOpts{AllTenants: cfg.OpenStack.AllTenants}).AllPages()
		return listErr
	})
	if err != nil {
		fetchServersStep.DoneWithError(err, "phase", "fetch_servers")
		return phaseError("list_servers", err)
	}
	srvList, err := servers.ExtractServers(srvPager)
	if err != nil {
		fetchServersStep.DoneWithError(err, "phase", "fetch_servers")
		return phaseError("extract_servers", err)
	}
	fetchServersStep.Done("phase", "fetch_servers", "count", len(srvList))
	log.Printf("Found %d servers", len(srvList))

	// Fetch projects
	log.Println("Fetching projects")
	fetchProjectsStep := logx.StepStart("sync_all_fetch_projects", "phase", "fetch_projects")
	var prjPager pagination.Page
	err = withAPIWatchdog("list_projects_all", func() error {
		var listErr error
		prjPager, listErr = projects.List(identityClient, nil).AllPages()
		return listErr
	})
	if err != nil {
		fetchProjectsStep.DoneWithError(err, "phase", "fetch_projects")
		return phaseError("list_projects", err)
	}
	prjList, err := projects.ExtractProjects(prjPager)
	if err != nil {
		fetchProjectsStep.DoneWithError(err, "phase", "fetch_projects")
		return phaseError("extract_projects", err)
	}
	fetchProjectsStep.Done("phase", "fetch_projects", "count", len(prjList))
	log.Printf("Found %d projects", len(prjList))

	// Fetch security groups for all projects using parallel workers
	fetchSecGrpsStep := logx.StepStart("sync_all_fetch_security_groups", "phase", "fetch_security_groups")
	allSecurityGroups, err := fetchSecurityGroupsParallel(networkClient, prjList, cfg)
	if err != nil {
		fetchSecGrpsStep.DoneWithError(err, "phase", "fetch_security_groups")
		return phaseError("fetch_security_groups", err)
	}
	fetchSecGrpsStep.Done("phase", "fetch_security_groups", "count", len(allSecurityGroups))
	log.Printf("Total security groups found: %d", len(allSecurityGroups))

	// Fetch volumes
	log.Printf("Fetching volumes (AllTenants: %v)", cfg.OpenStack.AllTenants)
	fetchVolumesStep := logx.StepStart("sync_all_fetch_volumes", "phase", "fetch_volumes")
	var volPager pagination.Page
	err = withAPIWatchdog("list_volumes_all", func() error {
		var listErr error
		volPager, listErr = volumes.List(blockStorageClient, volumes.ListOpts{AllTenants: cfg.OpenStack.AllTenants}).AllPages()
		return listErr
	})
	if err != nil {
		fetchVolumesStep.DoneWithError(err, "phase", "fetch_volumes")
		return phaseError("list_volumes", err)
	}
	volList, err := volumes.ExtractVolumes(volPager)
	if err != nil {
		fetchVolumesStep.DoneWithError(err, "phase", "fetch_volumes")
		return phaseError("extract_volumes", err)
	}
	fetchVolumesStep.Done("phase", "fetch_volumes", "count", len(volList))
	log.Printf("Found %d volumes", len(volList))

	// Build a map of security group name -> ID for each project (for server-secgrp lookups)
	sgNameToID := make(map[string]map[string]string) // projectID -> (sgName -> sgID)
	for _, sg := range allSecurityGroups {
		if sgNameToID[sg.ProjectID] == nil {
			sgNameToID[sg.ProjectID] = make(map[string]string)
		}
		sgNameToID[sg.ProjectID][sg.Group.Name] = sg.Group.ID
	}

	// Prepare statements
	log.Println("Preparing statements")
	prepareStep := logx.StepStart("sync_all_prepare_statements", "phase", "prepare_statements")
	stmtPrj, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.Projects+"(project_id, project_name) VALUES(?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "projects")
		return phaseError("prepare_projects_statement", err)
	}
	defer stmtPrj.Close()

	stmtSrv, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.Servers+"(server_id, server_name, project_id, ipv4_addr, status, image_id, image_name, flavor_id, flavor_name, metadata) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "servers")
		return phaseError("prepare_servers_statement", err)
	}
	defer stmtSrv.Close()

	stmtSG, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.SecGrps+"(secgrp_id, secgrp_name, project_id) VALUES(?, ?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "security_groups")
		return phaseError("prepare_security_groups_statement", err)
	}
	defer stmtSG.Close()

	stmtSGRule, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.SecGrpRules+"(rule_id, secgrp_id, direction, ethertype, protocol, port_range_min, port_range_max, remote_ip_prefix, remote_group_id) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "security_group_rules")
		return phaseError("prepare_security_group_rules_statement", err)
	}
	defer stmtSGRule.Close()

	stmtVol, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.Volumes+"(volume_id, volume_name, size_gb, volume_type, project_id) VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "volumes")
		return phaseError("prepare_volumes_statement", err)
	}
	defer stmtVol.Close()

	stmtSrvSG, err := tx.PrepareContext(ctx,
		"INSERT OR IGNORE INTO "+cfg.Tables.ServerSecGrps+"(server_id, secgrp_id) VALUES(?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "server_security_groups")
		return phaseError("prepare_server_security_groups_statement", err)
	}
	defer stmtSrvSG.Close()

	stmtSrvVol, err := tx.PrepareContext(ctx,
		"INSERT OR IGNORE INTO "+cfg.Tables.ServerVolumes+"(server_id, volume_id, device_path) VALUES(?, ?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "server_volumes")
		return phaseError("prepare_server_volumes_statement", err)
	}
	defer stmtSrvVol.Close()
	prepareStep.Done("phase", "prepare_statements")

	// Insert data
	insertProjectsStep := logx.StepStart("sync_all_insert_projects", "phase", "insert_projects", "count", len(prjList))
	log.Printf("Starting to insert %d projects", len(prjList))
	for i, p := range prjList {
		if err := ctx.Err(); err != nil {
			insertProjectsStep.DoneWithError(err, "phase", "insert_projects")
			return phaseError("insert_projects_context", err)
		}
		if _, err := stmtPrj.ExecContext(ctx, p.ID, p.Name); err != nil {
			insertProjectsStep.DoneWithError(err, "phase", "insert_projects", "project_id", p.ID, "index", i)
			return phaseError("insert_project", fmt.Errorf("project=%s id=%s index=%d: %w", p.Name, p.ID, i, err))
		}
		if (i+1)%100 == 0 {
			log.Printf("Inserted %d/%d projects", i+1, len(prjList))
		}
	}
	insertProjectsStep.Done("phase", "insert_projects", "count", len(prjList))

	insertServersStep := logx.StepStart("sync_all_insert_servers", "phase", "insert_servers", "count", len(srvList))
	log.Printf("Starting to insert %d servers", len(srvList))
	for i, s := range srvList {
		if err := ctx.Err(); err != nil {
			insertServersStep.DoneWithError(err, "phase", "insert_servers")
			return phaseError("insert_servers_context", err)
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

		// Extract image info
		var imageID, imageName string
		if s.Image != nil {
			if id, ok := s.Image["id"].(string); ok {
				imageID = id
			}
			if name, ok := s.Image["name"].(string); ok {
				imageName = name
			}
		}

		// Extract flavor info
		var flavorID, flavorName string
		if s.Flavor != nil {
			if id, ok := s.Flavor["id"].(string); ok {
				flavorID = id
			}
			if name, ok := s.Flavor["name"].(string); ok {
				flavorName = name
			}
		}

		// Serialize metadata to JSON
		var metadataJSON string
		if len(s.Metadata) > 0 {
			metadataBytes, err := json.Marshal(s.Metadata)
			if err != nil {
				log.Printf("Warning: failed to serialize metadata for server %s: %v", s.ID, err)
			} else {
				metadataJSON = string(metadataBytes)
			}
		}

		if _, err := stmtSrv.ExecContext(ctx, s.ID, s.Name, s.TenantID, ipv4Addr,
			s.Status, imageID, imageName, flavorID, flavorName, metadataJSON); err != nil {
			insertServersStep.DoneWithError(err, "phase", "insert_servers", "server_id", s.ID, "index", i)
			return phaseError("insert_server", fmt.Errorf("server=%s id=%s index=%d: %w", s.Name, s.ID, i, err))
		}

		if (i+1)%100 == 0 {
			log.Printf("Inserted %d/%d servers", i+1, len(srvList))
		}
	}
	insertServersStep.Done("phase", "insert_servers", "count", len(srvList))

	insertSecGroupsStep := logx.StepStart("sync_all_insert_security_groups", "phase", "insert_security_groups", "count", len(allSecurityGroups))
	log.Printf("Starting to insert %d security groups and their rules", len(allSecurityGroups))
	for i, sg := range allSecurityGroups {
		if err := ctx.Err(); err != nil {
			insertSecGroupsStep.DoneWithError(err, "phase", "insert_security_groups")
			return phaseError("insert_security_groups_context", err)
		}

		if _, err := stmtSG.ExecContext(ctx, sg.Group.ID, sg.Group.Name, sg.ProjectID); err != nil {
			insertSecGroupsStep.DoneWithError(err, "phase", "insert_security_groups", "secgrp_id", sg.Group.ID, "index", i)
			return phaseError("insert_security_group", fmt.Errorf("name=%s id=%s index=%d: %w", sg.Group.Name, sg.Group.ID, i, err))
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
				rule.RemoteIPPrefix,
				rule.RemoteGroupID); err != nil {
				insertSecGroupsStep.DoneWithError(err, "phase", "insert_security_groups", "secgrp_id", sg.Group.ID, "rule_id", rule.ID, "index", j)
				return phaseError("insert_security_group_rule", fmt.Errorf("rule_id=%s secgrp_id=%s index=%d: %w", rule.ID, sg.Group.ID, j, err))
			}
		}

		if (i+1)%10 == 0 {
			log.Printf("Inserted %d/%d security groups", i+1, len(allSecurityGroups))
		}
	}
	insertSecGroupsStep.Done("phase", "insert_security_groups", "count", len(allSecurityGroups))

	// Insert volumes
	insertVolumesStep := logx.StepStart("sync_all_insert_volumes", "phase", "insert_volumes", "count", len(volList))
	log.Printf("Starting to insert %d volumes", len(volList))
	for i, v := range volList {
		if err := ctx.Err(); err != nil {
			insertVolumesStep.DoneWithError(err, "phase", "insert_volumes")
			return phaseError("insert_volumes_context", err)
		}
		// Note: project_id is NULL as gophercloud Volume struct doesn't include it directly
		if _, err := stmtVol.ExecContext(ctx, v.ID, v.Name, v.Size, v.VolumeType, nil); err != nil {
			insertVolumesStep.DoneWithError(err, "phase", "insert_volumes", "volume_id", v.ID, "index", i)
			return phaseError("insert_volume", fmt.Errorf("name=%s id=%s index=%d: %w", v.Name, v.ID, i, err))
		}
		if (i+1)%100 == 0 {
			log.Printf("Inserted %d/%d volumes", i+1, len(volList))
		}
	}
	insertVolumesStep.Done("phase", "insert_volumes", "count", len(volList))

	// Insert server-security group mappings (after security groups are inserted)
	mappingSGStep := logx.StepStart("sync_all_insert_server_security_group_mappings", "phase", "insert_server_security_group_mappings")
	log.Println("Inserting server-security group mappings")
	serverSGCount := 0
	for _, s := range srvList {
		for _, sgMap := range s.SecurityGroups {
			sgName, ok := sgMap["name"].(string)
			if !ok {
				continue
			}
			// Look up security group ID by name in this project
			if projectSGs, exists := sgNameToID[s.TenantID]; exists {
				if sgID, found := projectSGs[sgName]; found {
					if _, err := stmtSrvSG.ExecContext(ctx, s.ID, sgID); err != nil {
						log.Printf("Warning: failed to insert server-secgrp mapping for server %s, secgrp %s: %v", s.ID, sgID, err)
					} else {
						serverSGCount++
					}
				}
			}
		}
	}
	log.Printf("Inserted %d server-security group mappings", serverSGCount)
	mappingSGStep.Done("phase", "insert_server_security_group_mappings", "count", serverSGCount)

	// Insert server-volume mappings (using AttachedVolumes from server data)
	mappingVolStep := logx.StepStart("sync_all_insert_server_volume_mappings", "phase", "insert_server_volume_mappings")
	log.Println("Inserting server-volume mappings")
	serverVolCount := 0
	for _, s := range srvList {
		for _, vol := range s.AttachedVolumes {
			// Note: AttachedVolume only has ID, device path is not available from this API
			if _, err := stmtSrvVol.ExecContext(ctx, s.ID, vol.ID, ""); err != nil {
				log.Printf("Warning: skipping server-volume mapping for server %s -> volume %s: volume not found in cache (may be deleted or from another project)", s.ID, vol.ID)
			} else {
				serverVolCount++
			}
		}
	}
	log.Printf("Inserted %d server-volume mappings", serverVolCount)
	mappingVolStep.Done("phase", "insert_server_volume_mappings", "count", serverVolCount)

	commitStep := logx.StepStart("sync_all_commit", "phase", "commit")
	log.Println("Committing transaction")
	if err := tx.Commit(); err != nil {
		commitStep.DoneWithError(err, "phase", "commit")
		return phaseError("commit", err)
	}
	commitStep.Done("phase", "commit")
	log.Println("Sync completed successfully")
	return nil
}

// findProjectByName looks up a project by name using partial matching (case-insensitive)
// Returns error if no match or multiple matches found
func findProjectByName(identityClient *gophercloud.ServiceClient, searchTerm string) (*projects.Project, error) {
	var allPages pagination.Page
	err := withAPIWatchdog("find_project_list_projects", func() error {
		var listErr error
		allPages, listErr = projects.List(identityClient, nil).AllPages()
		return listErr
	})
	if err != nil {
		return nil, phaseError("list_projects", err)
	}

	projectList, err := projects.ExtractProjects(allPages)
	if err != nil {
		return nil, phaseError("extract_projects", err)
	}

	searchLower := strings.ToLower(searchTerm)
	var matches []projects.Project

	for _, p := range projectList {
		if strings.Contains(strings.ToLower(p.Name), searchLower) {
			matches = append(matches, p)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no project found matching %q", searchTerm)
	case 1:
		// Warn if partial match (not exact)
		if strings.ToLower(matches[0].Name) != searchLower {
			log.Printf("⚠ Matched project %q (partial match for %q)", matches[0].Name, searchTerm)
		}
		return &matches[0], nil
	default:
		var names []string
		for _, m := range matches {
			names = append(names, m.Name)
		}
		return nil, fmt.Errorf("%q matches multiple projects:\n  - %s\nPlease specify a more precise project name",
			searchTerm, strings.Join(names, "\n  - "))
	}
}

// deleteProjectResources removes all cached data for a specific project
// Foreign key CASCADE will automatically handle junction tables
func deleteProjectResources(tx *sql.Tx, cfg *config.Config, projectID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Delete servers (CASCADE removes server_secgrps and server_volumes)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE project_id = ?", cfg.Tables.Servers), projectID); err != nil {
		return fmt.Errorf("failed to delete servers: %w", err)
	}

	// Delete security groups (CASCADE removes secgrp_rules and server_secgrps)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE project_id = ?", cfg.Tables.SecGrps), projectID); err != nil {
		return fmt.Errorf("failed to delete security groups: %w", err)
	}

	// Delete volumes (CASCADE removes server_volumes)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE project_id = ?", cfg.Tables.Volumes), projectID); err != nil {
		return fmt.Errorf("failed to delete volumes: %w", err)
	}

	return nil
}

// fetchServersByProject fetches servers for a single project
func fetchServersByProject(computeClient *gophercloud.ServiceClient, projectID string) ([]servers.Server, error) {
	var allPages pagination.Page
	err := withAPIWatchdog("list_servers_project_"+projectID, func() error {
		var listErr error
		allPages, listErr = servers.List(computeClient, servers.ListOpts{
			TenantID:   projectID,
			AllTenants: true,
		}).AllPages()
		return listErr
	})
	if err != nil {
		return nil, phaseError("list_servers", err)
	}

	serverList, err := servers.ExtractServers(allPages)
	if err != nil {
		return nil, phaseError("extract_servers", err)
	}

	return serverList, nil
}

// fetchSecurityGroupsByProject fetches security groups for a single project
func fetchSecurityGroupsByProject(networkClient *gophercloud.ServiceClient, projectID string) ([]groups.SecGroup, error) {
	var allPages pagination.Page
	err := withAPIWatchdog("list_security_groups_project_"+projectID, func() error {
		var listErr error
		allPages, listErr = groups.List(networkClient, groups.ListOpts{
			TenantID: projectID,
		}).AllPages()
		return listErr
	})
	if err != nil {
		return nil, phaseError("list_security_groups", err)
	}

	groupList, err := groups.ExtractGroups(allPages)
	if err != nil {
		return nil, phaseError("extract_security_groups", err)
	}

	return groupList, nil
}

// fetchVolumesByProject fetches volumes for a single project
func fetchVolumesByProject(blockStorageClient *gophercloud.ServiceClient, projectID string) ([]volumes.Volume, error) {
	var allPages pagination.Page
	err := withAPIWatchdog("list_volumes_project_"+projectID, func() error {
		var listErr error
		allPages, listErr = volumes.List(blockStorageClient, volumes.ListOpts{
			TenantID:   projectID,
			AllTenants: true,
		}).AllPages()
		return listErr
	})
	if err != nil {
		return nil, phaseError("list_volumes", err)
	}

	volumeList, err := volumes.ExtractVolumes(allPages)
	if err != nil {
		return nil, phaseError("extract_volumes", err)
	}

	return volumeList, nil
}

// SyncProject syncs resources for a specific project
func SyncProject(sqlDB *sql.DB, cfg *config.Config, projectName string) error {
	log.Printf("Starting project sync for: %s", projectName)

	// First verify OpenStack connectivity before making any database changes
	authStep := logx.StepStart("sync_project_auth", "phase", "auth", "project_query", projectName)
	computeClient, identityClient, networkClient, blockStorageClient, err := initOpenStackClients(cfg)
	if err != nil {
		authStep.DoneWithError(err, "phase", "auth")
		return phaseError("auth", err)
	}
	authStep.Done("phase", "auth")
	log.Println("Successfully authenticated with OpenStack services")

	// Find the project by name (with partial matching)
	findProjectStep := logx.StepStart("sync_project_resolve_name", "phase", "resolve_project_name", "project_query", projectName)
	targetProject, err := findProjectByName(identityClient, projectName)
	if err != nil {
		findProjectStep.DoneWithError(err, "phase", "resolve_project_name")
		return phaseError("resolve_project_name", err)
	}
	findProjectStep.Done("phase", "resolve_project_name", "project_id", targetProject.ID)
	log.Printf("Found project: %s (ID: %s)", targetProject.Name, targetProject.ID)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Start transaction for database operations
	txStep := logx.StepStart("sync_project_begin_tx", "phase", "begin_transaction", "project_id", targetProject.ID)
	tx, err := sqlDB.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
	if err != nil {
		txStep.DoneWithError(err, "phase", "begin_transaction")
		return phaseError("begin_transaction", err)
	}
	txStep.Done("phase", "begin_transaction")
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Warning: failed to rollback transaction: %v", err)
		}
	}()

	// Delete existing data for this project (CASCADE handles junction tables)
	clearStep := logx.StepStart("sync_project_delete_existing", "phase", "delete_project_resources", "project_id", targetProject.ID)
	log.Printf("Deleting existing data for project %s", targetProject.Name)
	if err := deleteProjectResources(tx, cfg, targetProject.ID); err != nil {
		clearStep.DoneWithError(err, "phase", "delete_project_resources")
		return phaseError("delete_project_resources", err)
	}
	clearStep.Done("phase", "delete_project_resources")

	// Fetch servers for this project
	fetchServersStep := logx.StepStart("sync_project_fetch_servers", "phase", "fetch_servers", "project_id", targetProject.ID)
	log.Printf("Fetching servers for project %s", targetProject.Name)
	srvList, err := fetchServersByProject(computeClient, targetProject.ID)
	if err != nil {
		fetchServersStep.DoneWithError(err, "phase", "fetch_servers")
		return phaseError("fetch_servers", err)
	}
	fetchServersStep.Done("phase", "fetch_servers", "count", len(srvList))
	log.Printf("Found %d servers", len(srvList))

	// Fetch security groups for this project
	fetchSecGrpsStep := logx.StepStart("sync_project_fetch_security_groups", "phase", "fetch_security_groups", "project_id", targetProject.ID)
	log.Printf("Fetching security groups for project %s", targetProject.Name)
	sgList, err := fetchSecurityGroupsByProject(networkClient, targetProject.ID)
	if err != nil {
		fetchSecGrpsStep.DoneWithError(err, "phase", "fetch_security_groups")
		return phaseError("fetch_security_groups", err)
	}
	fetchSecGrpsStep.Done("phase", "fetch_security_groups", "count", len(sgList))
	log.Printf("Found %d security groups", len(sgList))

	// Fetch volumes for this project
	fetchVolumesStep := logx.StepStart("sync_project_fetch_volumes", "phase", "fetch_volumes", "project_id", targetProject.ID)
	log.Printf("Fetching volumes for project %s", targetProject.Name)
	volList, err := fetchVolumesByProject(blockStorageClient, targetProject.ID)
	if err != nil {
		fetchVolumesStep.DoneWithError(err, "phase", "fetch_volumes")
		return phaseError("fetch_volumes", err)
	}
	fetchVolumesStep.Done("phase", "fetch_volumes", "count", len(volList))
	log.Printf("Found %d volumes", len(volList))

	// Build a map of security group name -> ID for server-secgrp lookups
	sgNameToID := make(map[string]string)
	for _, sg := range sgList {
		sgNameToID[sg.Name] = sg.ID
	}

	// Prepare statements
	log.Println("Preparing statements")
	prepareStep := logx.StepStart("sync_project_prepare_statements", "phase", "prepare_statements")
	stmtPrj, err := tx.PrepareContext(ctx,
		"INSERT OR REPLACE INTO "+cfg.Tables.Projects+"(project_id, project_name) VALUES(?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "projects")
		return phaseError("prepare_projects_statement", err)
	}
	defer stmtPrj.Close()

	stmtSrv, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.Servers+"(server_id, server_name, project_id, ipv4_addr, status, image_id, image_name, flavor_id, flavor_name, metadata) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "servers")
		return phaseError("prepare_servers_statement", err)
	}
	defer stmtSrv.Close()

	stmtSG, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.SecGrps+"(secgrp_id, secgrp_name, project_id) VALUES(?, ?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "security_groups")
		return phaseError("prepare_security_groups_statement", err)
	}
	defer stmtSG.Close()

	stmtSGRule, err := tx.PrepareContext(ctx,
		"INSERT INTO "+cfg.Tables.SecGrpRules+"(rule_id, secgrp_id, direction, ethertype, protocol, port_range_min, port_range_max, remote_ip_prefix, remote_group_id) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "security_group_rules")
		return phaseError("prepare_security_group_rules_statement", err)
	}
	defer stmtSGRule.Close()

	stmtVol, err := tx.PrepareContext(ctx,
		"INSERT OR REPLACE INTO "+cfg.Tables.Volumes+"(volume_id, volume_name, size_gb, volume_type, project_id) VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "volumes")
		return phaseError("prepare_volumes_statement", err)
	}
	defer stmtVol.Close()

	stmtSrvSG, err := tx.PrepareContext(ctx,
		"INSERT OR IGNORE INTO "+cfg.Tables.ServerSecGrps+"(server_id, secgrp_id) VALUES(?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "server_security_groups")
		return phaseError("prepare_server_security_groups_statement", err)
	}
	defer stmtSrvSG.Close()

	stmtSrvVol, err := tx.PrepareContext(ctx,
		"INSERT OR IGNORE INTO "+cfg.Tables.ServerVolumes+"(server_id, volume_id, device_path) VALUES(?, ?, ?)")
	if err != nil {
		prepareStep.DoneWithError(err, "phase", "prepare_statements", "statement", "server_volumes")
		return phaseError("prepare_server_volumes_statement", err)
	}
	defer stmtSrvVol.Close()
	prepareStep.Done("phase", "prepare_statements")

	// Insert project (UPSERT to update if already exists)
	insertProjectStep := logx.StepStart("sync_project_insert_project", "phase", "insert_project", "project_id", targetProject.ID)
	log.Printf("Inserting/updating project record for %s", targetProject.Name)
	if _, err := stmtPrj.ExecContext(ctx, targetProject.ID, targetProject.Name); err != nil {
		insertProjectStep.DoneWithError(err, "phase", "insert_project")
		return phaseError("insert_project", err)
	}
	insertProjectStep.Done("phase", "insert_project")

	// Insert servers
	insertServersStep := logx.StepStart("sync_project_insert_servers", "phase", "insert_servers", "count", len(srvList))
	log.Printf("Inserting %d servers", len(srvList))
	for i, s := range srvList {
		if err := ctx.Err(); err != nil {
			insertServersStep.DoneWithError(err, "phase", "insert_servers")
			return phaseError("insert_servers_context", err)
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

		// Extract image info
		var imageID, imageName string
		if s.Image != nil {
			if id, ok := s.Image["id"].(string); ok {
				imageID = id
			}
			if name, ok := s.Image["name"].(string); ok {
				imageName = name
			}
		}

		// Extract flavor info
		var flavorID, flavorName string
		if s.Flavor != nil {
			if id, ok := s.Flavor["id"].(string); ok {
				flavorID = id
			}
			if name, ok := s.Flavor["name"].(string); ok {
				flavorName = name
			}
		}

		// Serialize metadata to JSON
		var metadataJSON string
		if s.Metadata != nil && len(s.Metadata) > 0 {
			if jsonBytes, err := json.Marshal(s.Metadata); err == nil {
				metadataJSON = string(jsonBytes)
			}
		}

		if _, err := stmtSrv.ExecContext(ctx, s.ID, s.Name, s.TenantID, ipv4Addr, s.Status, imageID, imageName, flavorID, flavorName, metadataJSON); err != nil {
			insertServersStep.DoneWithError(err, "phase", "insert_servers", "server_id", s.ID, "index", i)
			return phaseError("insert_server", fmt.Errorf("server=%s id=%s index=%d: %w", s.Name, s.ID, i, err))
		}
		if (i+1)%100 == 0 {
			log.Printf("Inserted %d/%d servers", i+1, len(srvList))
		}
	}
	insertServersStep.Done("phase", "insert_servers", "count", len(srvList))
	log.Printf("Inserted %d servers", len(srvList))

	// Insert security groups and rules
	insertSecGrpsStep := logx.StepStart("sync_project_insert_security_groups", "phase", "insert_security_groups", "count", len(sgList))
	log.Printf("Inserting %d security groups", len(sgList))
	ruleCount := 0
	for i, sg := range sgList {
		if err := ctx.Err(); err != nil {
			insertSecGrpsStep.DoneWithError(err, "phase", "insert_security_groups")
			return phaseError("insert_security_groups_context", err)
		}

		if _, err := stmtSG.ExecContext(ctx, sg.ID, sg.Name, sg.ProjectID); err != nil {
			insertSecGrpsStep.DoneWithError(err, "phase", "insert_security_groups", "secgrp_id", sg.ID, "index", i)
			return phaseError("insert_security_group", fmt.Errorf("name=%s id=%s index=%d: %w", sg.Name, sg.ID, i, err))
		}

		// Insert rules for this security group
		for _, rule := range sg.Rules {
			if _, err := stmtSGRule.ExecContext(ctx, rule.ID, sg.ID, rule.Direction, rule.EtherType,
				rule.Protocol, rule.PortRangeMin, rule.PortRangeMax, rule.RemoteIPPrefix, rule.RemoteGroupID); err != nil {
				insertSecGrpsStep.DoneWithError(err, "phase", "insert_security_groups", "secgrp_id", sg.ID, "rule_id", rule.ID)
				return phaseError("insert_security_group_rule", fmt.Errorf("rule_id=%s secgrp_id=%s: %w", rule.ID, sg.ID, err))
			}
			ruleCount++
		}

		if (i+1)%10 == 0 {
			log.Printf("Inserted %d/%d security groups", i+1, len(sgList))
		}
	}
	insertSecGrpsStep.Done("phase", "insert_security_groups", "count", len(sgList), "rule_count", ruleCount)
	log.Printf("Inserted %d security groups with %d rules", len(sgList), ruleCount)

	// Insert volumes
	insertVolumesStep := logx.StepStart("sync_project_insert_volumes", "phase", "insert_volumes", "count", len(volList))
	log.Printf("Inserting %d volumes", len(volList))
	for i, v := range volList {
		if err := ctx.Err(); err != nil {
			insertVolumesStep.DoneWithError(err, "phase", "insert_volumes")
			return phaseError("insert_volumes_context", err)
		}
		// Note: gophercloud Volume struct doesn't have ProjectID field, using targetProject.ID
		if _, err := stmtVol.ExecContext(ctx, v.ID, v.Name, v.Size, v.VolumeType, targetProject.ID); err != nil {
			insertVolumesStep.DoneWithError(err, "phase", "insert_volumes", "volume_id", v.ID, "index", i)
			return phaseError("insert_volume", fmt.Errorf("name=%s id=%s index=%d: %w", v.Name, v.ID, i, err))
		}
		if (i+1)%100 == 0 {
			log.Printf("Inserted %d/%d volumes", i+1, len(volList))
		}
	}
	insertVolumesStep.Done("phase", "insert_volumes", "count", len(volList))
	log.Printf("Inserted %d volumes", len(volList))

	// Insert server-security group mappings
	mappingSGStep := logx.StepStart("sync_project_insert_server_security_group_mappings", "phase", "insert_server_security_group_mappings")
	log.Println("Inserting server-security group mappings")
	srvSGCount := 0
	for _, s := range srvList {
		for _, sg := range s.SecurityGroups {
			sgName := sg["name"].(string)
			if sgID, ok := sgNameToID[sgName]; ok {
				if _, err := stmtSrvSG.ExecContext(ctx, s.ID, sgID); err != nil {
					log.Printf("Warning: failed to insert server-secgrp mapping for server %s, secgrp %s: %v", s.ID, sgName, err)
				} else {
					srvSGCount++
				}
			}
		}
	}
	log.Printf("Inserted %d server-security group mappings", srvSGCount)
	mappingSGStep.Done("phase", "insert_server_security_group_mappings", "count", srvSGCount)

	// Insert server-volume mappings (using AttachedVolumes from server data)
	mappingVolStep := logx.StepStart("sync_project_insert_server_volume_mappings", "phase", "insert_server_volume_mappings")
	log.Println("Inserting server-volume mappings")
	serverVolCount := 0
	for _, s := range srvList {
		for _, vol := range s.AttachedVolumes {
			if _, err := stmtSrvVol.ExecContext(ctx, s.ID, vol.ID, ""); err != nil {
				log.Printf("Warning: skipping server-volume mapping for server %s -> volume %s: volume not found in cache (may be deleted or from another project)", s.ID, vol.ID)
			} else {
				serverVolCount++
			}
		}
	}
	log.Printf("Inserted %d server-volume mappings", serverVolCount)
	mappingVolStep.Done("phase", "insert_server_volume_mappings", "count", serverVolCount)

	commitStep := logx.StepStart("sync_project_commit", "phase", "commit", "project_id", targetProject.ID)
	log.Println("Committing transaction")
	if err := tx.Commit(); err != nil {
		commitStep.DoneWithError(err, "phase", "commit")
		return phaseError("commit", err)
	}
	commitStep.Done("phase", "commit")
	log.Printf("Sync completed successfully for project %s", targetProject.Name)
	return nil
}
