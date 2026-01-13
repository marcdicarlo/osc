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

var serversFullOutput bool

// serversCmd represents the servers command
var serversCmd = &cobra.Command{
	Use:   "servers",
	Short: "List all OpenStack servers",
	Long: `List all OpenStack servers.

Examples:

# list all openstack servers
osc list servers

# list servers with security groups
osc list servers --full

# list servers in projects containing a string
osc list servers -p "prod"    # matches: prod-app1, prod-app2, production
osc list servers -p "eta"     # matches: hc_zeta_project, hc_eta_project, hc_beta_project

# list servers in different output formats
osc list servers -o json
osc list servers -o csv
osc list servers --full -o json  # with security groups in JSON format
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
		if err := Servers(db, cfg); err != nil {
			log.Fatalf("Failed to list servers: %v", err)
		}
	},
}

func init() {
	listCmd.AddCommand(serversCmd)
	serversCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Filter servers by project name (shows projects containing this string)")
	serversCmd.Flags().BoolVarP(&serversFullOutput, "full", "f", false, "Include security groups attached to each server")
}

// Servers reads and outputs server/project data.
func Servers(db *sql.DB, cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Build query based on --full flag
	var query string
	if serversFullOutput {
		// Full query with security groups using GROUP_CONCAT
		query = `SELECT s.server_name, s.server_id, p.project_name, COALESCE(s.ipv4_addr, ''),
		         COALESCE(GROUP_CONCAT(sg.secgrp_id || ' (' || sg.secgrp_name || ')', ', '), '')
		FROM ` + cfg.Tables.Servers + ` s
		JOIN ` + cfg.Tables.Projects + ` p USING (project_id)
		LEFT JOIN ` + cfg.Tables.ServerSecGrps + ` ssg ON s.server_id = ssg.server_id
		LEFT JOIN ` + cfg.Tables.SecGrps + ` sg ON ssg.secgrp_id = sg.secgrp_id
		GROUP BY s.server_id
		ORDER BY s.server_name;`
	} else {
		// Basic query without security groups
		query = `SELECT s.server_name, s.server_id, p.project_name, COALESCE(s.ipv4_addr, '')
		FROM ` + cfg.Tables.Servers + ` s
		JOIN ` + cfg.Tables.Projects + ` p USING (project_id)
		ORDER BY s.server_name;`
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Collect the data
	var data [][]string
	for rows.Next() {
		if serversFullOutput {
			var name, id, pname, ipv4, secgrps string
			if err := rows.Scan(&name, &id, &pname, &ipv4, &secgrps); err != nil {
				return err
			}
			data = append(data, []string{name, id, pname, ipv4, secgrps})
		} else {
			var name, id, pname, ipv4 string
			if err := rows.Scan(&name, &id, &pname, &ipv4); err != nil {
				return err
			}
			data = append(data, []string{name, id, pname, ipv4})
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Apply project filtering
	pf := filter.New(projectFilter, cfg)
	filteredData, matchedProjectsMap := pf.MatchProjects(data, 2) // 2 is the index of project_name in our data

	// Create the output formatter
	formatter, err := output.NewFormatter(outputFormat, os.Stdout)
	if err != nil {
		return err
	}

	// Build headers based on --full flag
	headers := []string{"Server Name", "Server ID", "Project Name", "IPv4 Address"}
	if serversFullOutput {
		headers = append(headers, "Security Groups")
	}

	// Prepare output data with headers and filtering info
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
