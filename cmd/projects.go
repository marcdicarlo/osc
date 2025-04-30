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
	"github.com/olekukonko/tablewriter"

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

`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration from YAML
		cfg, err := config.Load("config.yaml")
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		// initalize the database
		db, err := db.InitDB(cfg)
		if err != nil {
			log.Fatalf("DB init failed: %v", err)
		}
		defer db.Close()
		// Print the projects table
		Print(db, cfg)
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

// Print reads and outputs server/project data.
func Print(db *sql.DB, cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// query all project names and project ids from the database projects table
	query := `SELECT project_id, project_name FROM ` + cfg.Tables.Projects

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Initialize table
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Project ID", "Project Name"})
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetTablePadding("\t")

	for rows.Next() {
		var pid, pname string
		if err := rows.Scan(&pid, &pname); err != nil {
			return err
		}
		table.Append([]string{pid, pname})
	}

	table.Render()
	return rows.Err()
}
