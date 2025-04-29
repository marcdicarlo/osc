/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Display one or more resouces",
	Long: `Display one or more resouces.

  Prints a table of the most important information about specified resources. 

Available resources:
    projects  returns a list of OpenStack projects
    servers  returns all OpenStack servers
    secgrps returns a list of all security groups

Examples:

# list all openstack projects
osc list projects

# list all openstack servers
osc list servers

# list servers for a specified a project glob
oec list servers -p <project_glob>

# list all security groups for a project glob
osc list secgrps -p <project_glob>`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Fatal("List must be called with a subcommand")
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// listCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
