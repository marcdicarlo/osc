/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"log"

	"github.com/marcdicarlo/osc/internal/config"
	"github.com/marcdicarlo/osc/internal/db"
	"github.com/marcdicarlo/osc/internal/openstack"
	"github.com/spf13/cobra"
)

// syncProjectCmd represents the project command
var syncProjectCmd = &cobra.Command{
	Use:   "project <project-name>",
	Short: "Sync resources for a specific project",
	Long: `Sync servers, security groups, and volumes for a specific OpenStack project.

The project name supports partial matching (case-insensitive). If multiple projects
match, you'll be asked to be more specific.

Examples:

  # Sync all resources for a project (exact or partial match)
  osc sync project production-web
  osc sync project prod
  `,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := args[0]
		cfg, err := config.Load("config.yaml")
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		database, err := db.InitDB(cfg)
		if err != nil {
			log.Fatalf("Failed to init db: %v", err)
		}
		defer database.Close()

		if err := openstack.SyncProject(database, cfg, projectName); err != nil {
			log.Fatalf("Error syncing project: %v", err)
		}
	},
}

func init() {
	syncCmd.AddCommand(syncProjectCmd)
}
