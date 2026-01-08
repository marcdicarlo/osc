package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show details for a specific resource",
	Long: `Show detailed information for a specific OpenStack resource.

Available resources:
    server  Show detailed information for a specific server
    secgrp  Show detailed information for a specific security group

Examples:

# show details for a server by name
osc show server my-server-name

# show server details in a specific project
osc show server my-server-name -p prod

# show security group details
osc show secgrp web-servers

# show details in different formats
osc show server my-server-name -o json
osc show secgrp web-servers -o csv`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Fatal("Show must be called with a subcommand")
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
