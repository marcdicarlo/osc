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
	"github.com/marcdicarlo/osc/internal/output"
	"github.com/spf13/cobra"
)

// projectsCmd represents the projects command
var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all OpenStack projects",
	Long: `List all OpenStack projects.

Examples:

# list all openstack projects
osc list projects

# list projects in JSON format
osc list projects -o json

# list projects in CSV format
osc list projects -o csv
`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration from YAML
		cfg, err := config.Load("config.yaml")
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		// initialize the database
		db, err := db.InitDB(cfg)
		if err != nil {
			log.Fatalf("DB init failed: %v", err)
		}
		defer db.Close()
		// Print the projects
		if err := Print(db, cfg); err != nil {
			log.Fatalf("Failed to print projects: %v", err)
		}
	},
}

func init() {
	listCmd.AddCommand(projectsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// projectsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// projectsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Print reads and outputs project data.
func Print(db *sql.DB, cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Query all project names and project ids from the database projects table
	query := `SELECT project_id, project_name FROM ` + cfg.Tables.Projects

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Collect the data
	var data [][]string
	for rows.Next() {
		var pid, pname string
		if err := rows.Scan(&pid, &pname); err != nil {
			return err
		}
		data = append(data, []string{pid, pname})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Create the output formatter
	formatter, err := output.NewFormatter(outputFormat, os.Stdout)
	if err != nil {
		return err
	}

	// Format and output the data
	outputData := output.NewOutputData(
		[]string{"Project ID", "Project Name"},
		data,
	)

	return formatter.Format(outputData)
}
