/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/marcdicarlo/osc/internal/config"
	"github.com/marcdicarlo/osc/internal/db"
	"github.com/marcdicarlo/osc/internal/filter"
	"github.com/marcdicarlo/osc/internal/output"
	"github.com/spf13/cobra"
)

// secgrpsCmd represents the secgrps command
var (
	rules       bool
	fullOutput  bool
	sortGrouped bool
	secgrpsCmd = &cobra.Command{
		Use:   "secgrps",
		Short: "List all OpenStack security groups and rules",
		Long: `List all OpenStack security groups and optionally their rules.

Examples:

# list all security groups
osc list secgrps

# list security groups for projects matching a pattern
osc list secgrps -p "prod"

# list security groups and their rules
osc list secgrps -r

# list security groups with full rule details (ethertype, remote groups)
osc list secgrps -r --full
osc list secgrps -r -f

# list security groups grouped with their rules
osc list secgrps -r -s
osc list secgrps -r --sort

# combine sort with full output
osc list secgrps -r -f -s
osc list secgrps -r --full --sort -o json

# list security groups and rules in different formats
osc list secgrps -r -o json
osc list secgrps -r --full -o json
osc list secgrps -r -f -o csv
osc list secgrps -p "prod" -r -o json
`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load("config.yaml")
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}
			db, err := db.InitDB(cfg)
			if err != nil {
				log.Fatalf("Failed to init db: %v", err)
			}
			defer db.Close()
			if err := Secgrps(db, cfg); err != nil {
				log.Fatalf("Failed to list security groups: %v", err)
			}
		},
	}
)

func init() {
	listCmd.AddCommand(secgrpsCmd)
	secgrpsCmd.Flags().BoolVarP(&rules, "rules", "r", false, "Show rules for each security group")
	secgrpsCmd.Flags().BoolVarP(&fullOutput, "full", "f", false, "Show full rule details including ethertype and remote group IDs (requires -r)")
	secgrpsCmd.Flags().BoolVarP(&sortGrouped, "sort", "s", false, "Group security groups with their rules together (requires -r)")
	secgrpsCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Filter security groups by project name (shows projects containing this string)")
}

// Secgrps reads and outputs security group and rule data.
func Secgrps(db *sql.DB, cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Validate flag combinations
	if fullOutput && !rules {
		return fmt.Errorf("--full flag requires --rules flag")
	}
	if sortGrouped && !fullOutput {
		return fmt.Errorf("--sort flag requires --full flag")
	}

	// Build the base query for security groups
	var query string
	if rules && fullOutput {
		// Full output mode: include ethertype and remote_group_id with group name resolution
		query = `SELECT
			s.secgrp_name as name,
			s.secgrp_id as id,
			s.secgrp_id as parent_id,
			s.project_id,
			p.project_name,
			'security-group' as resource_type,
			'' as direction,
			'' as protocol,
			'' as port_range,
			'' as remote_ip,
			'' as ethertype,
			'' as remote_group_id,
			'' as remote_group_name
		FROM ` + cfg.Tables.SecGrps + ` s
		JOIN ` + cfg.Tables.Projects + ` p USING (project_id)
		UNION ALL
		SELECT
			r.rule_id as name,
			r.rule_id as id,
			r.secgrp_id as parent_id,
			s.project_id,
			p.project_name,
			'security-group-rule' as resource_type,
			r.direction,
			COALESCE(r.protocol, 'any') as protocol,
			CASE
				WHEN r.port_range_min IS NULL AND r.port_range_max IS NULL THEN 'any'
				WHEN r.port_range_min = r.port_range_max THEN CAST(r.port_range_min AS TEXT)
				ELSE CAST(r.port_range_min AS TEXT) || '-' || CAST(r.port_range_max AS TEXT)
			END as port_range,
			COALESCE(r.remote_ip_prefix, 'any') as remote_ip,
			r.ethertype,
			COALESCE(r.remote_group_id, '') as remote_group_id,
			COALESCE(sg_remote.secgrp_name, '') as remote_group_name
		FROM ` + cfg.Tables.SecGrpRules + ` r
		JOIN ` + cfg.Tables.SecGrps + ` s ON r.secgrp_id = s.secgrp_id
		JOIN ` + cfg.Tables.Projects + ` p ON s.project_id = p.project_id
		LEFT JOIN ` + cfg.Tables.SecGrps + ` sg_remote ON r.remote_group_id = sg_remote.secgrp_id
		`
		// Add ORDER BY clause based on sort flag
		if sortGrouped {
			query += "ORDER BY parent_id, resource_type, name;"
		} else {
			query += "ORDER BY resource_type DESC, name;"
		}
	} else if rules {
		// Basic rules mode
		query = `SELECT
			s.secgrp_name as name,
			s.secgrp_id as id,
			s.secgrp_id as parent_id,
			s.project_id,
			p.project_name,
			'security-group' as resource_type,
			'' as direction,
			'' as protocol,
			'' as port_range,
			'' as remote_ip
		FROM ` + cfg.Tables.SecGrps + ` s
		JOIN ` + cfg.Tables.Projects + ` p USING (project_id)
		UNION ALL
		SELECT
			r.rule_id as name,
			r.rule_id as id,
			r.secgrp_id as parent_id,
			s.project_id,
			p.project_name,
			'security-group-rule' as resource_type,
			r.direction,
			COALESCE(r.protocol, 'any') as protocol,
			CASE
				WHEN r.port_range_min IS NULL AND r.port_range_max IS NULL THEN 'any'
				WHEN r.port_range_min = r.port_range_max THEN CAST(r.port_range_min AS TEXT)
				ELSE CAST(r.port_range_min AS TEXT) || '-' || CAST(r.port_range_max AS TEXT)
			END as port_range,
			COALESCE(r.remote_ip_prefix, 'any') as remote_ip
		FROM ` + cfg.Tables.SecGrpRules + ` r
		JOIN ` + cfg.Tables.SecGrps + ` s ON r.secgrp_id = s.secgrp_id
		JOIN ` + cfg.Tables.Projects + ` p ON s.project_id = p.project_id
		`
		// Add ORDER BY clause based on sort flag
		if sortGrouped {
			query += "ORDER BY parent_id, resource_type, name;"
		} else {
			query += "ORDER BY resource_type DESC, name;"
		}
	} else {
		// Security groups only
		query = `SELECT
			s.secgrp_name as name,
			s.secgrp_id as id,
			s.project_id,
			p.project_name,
			'security-group' as resource_type,
			'' as direction,
			'' as protocol,
			'' as port_range,
			'' as remote_ip
		FROM ` + cfg.Tables.SecGrps + ` s
		JOIN ` + cfg.Tables.Projects + ` p USING (project_id)
		ORDER BY s.secgrp_name;`
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Collect the data
	var data [][]string
	for rows.Next() {
		var name, id, parentID, pid, pname, rtype, direction, protocol, portRange, remoteIP string
		var ethertype, remoteGroupID, remoteGroupName string

		if rules && fullOutput {
			if err := rows.Scan(&name, &id, &parentID, &pid, &pname, &rtype, &direction, &protocol, &portRange, &remoteIP, &ethertype, &remoteGroupID, &remoteGroupName); err != nil {
				return err
			}
			row := []string{name, id, parentID, pid, pname, rtype}
			row = append(row, direction, protocol, portRange, remoteIP, ethertype)
			// Combine remote_group_id and remote_group_name for display
			if remoteGroupID != "" && remoteGroupName != "" {
				row = append(row, remoteGroupID+" ("+remoteGroupName+")")
			} else if remoteGroupID != "" {
				row = append(row, remoteGroupID)
			} else {
				row = append(row, "")
			}
			data = append(data, row)
		} else if rules {
			if err := rows.Scan(&name, &id, &parentID, &pid, &pname, &rtype, &direction, &protocol, &portRange, &remoteIP); err != nil {
				return err
			}
			row := []string{name, id, parentID, pid, pname, rtype}
			// Rule details are NEVER appended in basic rules mode (-r without --full)
			// This mode only shows that rules exist (via resource_type), not their details
			data = append(data, row)
		} else {
			// Security groups only - no parent_id column
			if err := rows.Scan(&name, &id, &pid, &pname, &rtype, &direction, &protocol, &portRange, &remoteIP); err != nil {
				return err
			}
			row := []string{name, id, pid, pname, rtype}
			data = append(data, row)
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Apply project filtering
	// When rules flag is set, parent_id column is added so project_name is at index 4
	// Otherwise project_name is at index 3
	projectNameIndex := 3
	if rules {
		projectNameIndex = 4
	}
	pf := filter.New(projectFilter, cfg)
	filteredData, matchedProjectsMap := pf.MatchProjects(data, projectNameIndex)

	// Create the output formatter
	formatter, err := output.NewFormatter(outputFormat, os.Stdout)
	if err != nil {
		return err
	}

	// Prepare output data with headers
	var headers []string
	if rules {
		headers = []string{"Name", "ID", "Parent ID", "Project ID", "Project Name", "Resource Type"}
		if fullOutput {
			headers = append(headers, "Direction", "Protocol", "Port Range", "Remote IP", "Ethertype", "Remote Group")
		}
	} else {
		headers = []string{"Name", "ID", "Project ID", "Project Name", "Resource Type"}
	}
	outputData := output.NewOutputData(headers, filteredData)

	// Add filtering metadata if filtering was applied
	if pf.GetActiveFilter() != "" {
		// Convert matched projects map to slice
		var matchedProjects []string
		for project := range matchedProjectsMap {
			matchedProjects = append(matchedProjects, project)
		}
		outputData.WithFilterInfo(matchedProjects)
	}

	return formatter.Format(outputData)
}
