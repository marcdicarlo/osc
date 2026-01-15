/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marcdicarlo/osc/internal/config"
	"github.com/marcdicarlo/osc/internal/db"
	"github.com/marcdicarlo/osc/internal/drift"
	"github.com/marcdicarlo/osc/internal/filter"
	"github.com/marcdicarlo/osc/internal/output"
	"github.com/spf13/cobra"
)

var (
	// driftGeneratePath is the path for the generate subcommand
	driftGeneratePath string
)

// driftGenerateCmd represents the drift generate command
var driftGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate truth files from osc cache",
	Long: `Generate truth files (servers.json, secgrps.json) for each project directory.

This command scans all project directories in the specified path and creates
truth files based on the osc cached data, using the folder name as the project filter.

Example:
    osc drift generate --path ./tmp

This will create:
    ./tmp/project1/truth/servers.json
    ./tmp/project1/truth/secgrps.json
    ./tmp/project2/truth/servers.json
    ./tmp/project2/truth/secgrps.json
    ...`,
	RunE: runDriftGenerate,
}

func init() {
	driftCmd.AddCommand(driftGenerateCmd)

	driftGenerateCmd.Flags().StringVarP(&driftGeneratePath, "path", "p", "", "Path to directory containing project folders (required)")
	driftGenerateCmd.MarkFlagRequired("path")
}

func runDriftGenerate(cmd *cobra.Command, args []string) error {
	// Load config and initialize database
	cfg, err := config.Load("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		return fmt.Errorf("failed to init db: %w", err)
	}
	defer database.Close()

	// Discover project directories
	projects, err := drift.DiscoverProjects(driftGeneratePath)
	if err != nil {
		// If no projects found, try to use direct subdirectories
		entries, readErr := os.ReadDir(driftGeneratePath)
		if readErr != nil {
			return fmt.Errorf("failed to read path: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				projectPath := filepath.Join(driftGeneratePath, entry.Name())
				projects = append(projects, drift.ProjectDir{
					Name:      entry.Name(),
					BasePath:  projectPath,
					StatePath: filepath.Join(projectPath, "state"),
					TruthPath: filepath.Join(projectPath, "truth"),
				})
			}
		}

		if len(projects) == 0 {
			return fmt.Errorf("no project directories found in %s", driftGeneratePath)
		}
	}

	successCount := 0
	for _, project := range projects {
		fmt.Printf("Generating truth files for project: %s\n", project.Name)

		// Ensure truth directory exists
		if err := drift.EnsureProjectDirs(project.BasePath); err != nil {
			fmt.Printf("  Warning: failed to create directories: %v\n", err)
			continue
		}

		// Generate servers.json
		serversPath := filepath.Join(project.TruthPath, "servers.json")
		if err := generateServersJSON(database, cfg, project.Name, serversPath); err != nil {
			fmt.Printf("  Warning: failed to generate servers.json: %v\n", err)
		} else {
			fmt.Printf("  Created: %s\n", serversPath)
		}

		// Generate secgrps.json
		secgrpsPath := filepath.Join(project.TruthPath, "secgrps.json")
		if err := generateSecgrpsJSON(database, cfg, project.Name, secgrpsPath); err != nil {
			fmt.Printf("  Warning: failed to generate secgrps.json: %v\n", err)
		} else {
			fmt.Printf("  Created: %s\n", secgrpsPath)
		}

		successCount++
	}

	fmt.Printf("\nGenerated truth files for %d/%d projects\n", successCount, len(projects))
	return nil
}

// generateServersJSON generates the servers.json file for a project
func generateServersJSON(database *sql.DB, cfg *config.Config, projectName, outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Query with security groups (always include for drift detection)
	query := `SELECT s.server_name, s.server_id, p.project_name, COALESCE(s.ipv4_addr, ''),
	         COALESCE(GROUP_CONCAT(sg.secgrp_name, ', '), '')
	FROM ` + cfg.Tables.Servers + ` s
	JOIN ` + cfg.Tables.Projects + ` p USING (project_id)
	LEFT JOIN ` + cfg.Tables.ServerSecGrps + ` ssg ON s.server_id = ssg.server_id
	LEFT JOIN ` + cfg.Tables.SecGrps + ` sg ON ssg.secgrp_id = sg.secgrp_id
	GROUP BY s.server_id
	ORDER BY s.server_name;`

	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var data [][]string
	for rows.Next() {
		var name, id, pname, ipv4, secgrps string
		if err := rows.Scan(&name, &id, &pname, &ipv4, &secgrps); err != nil {
			return err
		}
		data = append(data, []string{name, id, pname, ipv4, secgrps})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Filter to only this project
	pf := filter.New(projectName, cfg)
	filteredData, matchedProjectsMap := pf.MatchProjects(data, 2)

	// If no data matches this project, create empty output
	if len(filteredData) == 0 {
		return writeEmptyJSON(outputPath, []string{"Server Name", "Server ID", "Project Name", "IPv4 Address", "Security Groups"})
	}

	// Write to file
	var buf bytes.Buffer
	formatter, err := output.NewFormatter("json", &buf)
	if err != nil {
		return err
	}

	headers := []string{"Server Name", "Server ID", "Project Name", "IPv4 Address", "Security Groups"}
	outputData := output.NewOutputData(headers, filteredData)

	var matchedProjects []string
	for project := range matchedProjectsMap {
		matchedProjects = append(matchedProjects, project)
	}
	outputData.WithFilterInfo(matchedProjects)

	if err := formatter.Format(outputData); err != nil {
		return err
	}

	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}

// generateSecgrpsJSON generates the secgrps.json file for a project
func generateSecgrpsJSON(database *sql.DB, cfg *config.Config, projectName, outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Query security groups with rules
	query := `SELECT
		sg.secgrp_name, sg.secgrp_id, sg.project_id, p.project_name, 'security-group' as resource_type
	FROM ` + cfg.Tables.SecGrps + ` sg
	JOIN ` + cfg.Tables.Projects + ` p ON sg.project_id = p.project_id
	UNION ALL
	SELECT
		r.rule_id, r.rule_id, sg.project_id, p.project_name, 'security-group-rule' as resource_type
	FROM ` + cfg.Tables.SecGrpRules + ` r
	JOIN ` + cfg.Tables.SecGrps + ` sg ON r.secgrp_id = sg.secgrp_id
	JOIN ` + cfg.Tables.Projects + ` p ON sg.project_id = p.project_id
	ORDER BY resource_type DESC, 1;`

	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var data [][]string
	for rows.Next() {
		var name, id, projectID, pname, resourceType string
		if err := rows.Scan(&name, &id, &projectID, &pname, &resourceType); err != nil {
			return err
		}
		data = append(data, []string{name, id, projectID, pname, resourceType})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Filter to only this project
	pf := filter.New(projectName, cfg)
	filteredData, matchedProjectsMap := pf.MatchProjects(data, 3) // project_name is at index 3

	if len(filteredData) == 0 {
		return writeEmptyJSON(outputPath, []string{"Name", "ID", "Project ID", "Project Name", "Resource Type"})
	}

	// Write to file
	var buf bytes.Buffer
	formatter, err := output.NewFormatter("json", &buf)
	if err != nil {
		return err
	}

	headers := []string{"Name", "ID", "Project ID", "Project Name", "Resource Type"}
	outputData := output.NewOutputData(headers, filteredData)

	var matchedProjects []string
	for project := range matchedProjectsMap {
		matchedProjects = append(matchedProjects, project)
	}
	outputData.WithFilterInfo(matchedProjects)

	if err := formatter.Format(outputData); err != nil {
		return err
	}

	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}

// writeEmptyJSON writes an empty JSON output file
func writeEmptyJSON(outputPath string, headers []string) error {
	var buf bytes.Buffer
	formatter, err := output.NewFormatter("json", &buf)
	if err != nil {
		return err
	}

	outputData := output.NewOutputData(headers, [][]string{})
	if err := formatter.Format(outputData); err != nil {
		return err
	}

	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}
