/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"database/sql"
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
	rules      bool
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

# list security groups and rules in different formats
osc list secgrps -r -o json
osc list secgrps -r -o csv
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
	secgrpsCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Filter security groups by project name (shows projects containing this string)")
}

// Secgrps reads and outputs security group and rule data.
func Secgrps(db *sql.DB, cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Build the base query for security groups
	query := `SELECT 
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
	JOIN ` + cfg.Tables.Projects + ` p USING (project_id)`

	// If rules are requested, union with rules query
	if rules {
		query += `
		UNION ALL
		SELECT 
			r.rule_id as name,
			r.secgrp_id as id,
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
		ORDER BY resource_type DESC, name;`
	} else {
		query += ` ORDER BY s.secgrp_name;`
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Collect the data
	var data [][]string
	for rows.Next() {
		var name, id, pid, pname, rtype, direction, protocol, portRange, remoteIP string
		if err := rows.Scan(&name, &id, &pid, &pname, &rtype, &direction, &protocol, &portRange, &remoteIP); err != nil {
			return err
		}
		row := []string{name, id, pid, pname, rtype}
		if rules {
			row = append(row, direction, protocol, portRange, remoteIP)
		}
		data = append(data, row)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Apply project filtering
	pf := filter.New(projectFilter, cfg)
	filteredData, matchedProjectsMap := pf.MatchProjects(data, 3) // 3 is the index of project_name

	// Create the output formatter
	formatter, err := output.NewFormatter(outputFormat, os.Stdout)
	if err != nil {
		return err
	}

	// Prepare output data with headers
	headers := []string{"Name", "ID", "Project ID", "Project Name", "Resource Type"}
	if rules {
		headers = append(headers, "Direction", "Protocol", "Port Range", "Remote IP")
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
