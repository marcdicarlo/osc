/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// driftCmd represents the drift command
var driftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Detect drift between Terraform state and OpenStack",
	Long: `Compare Terraform state files with osc cached data to identify infrastructure drift.

The drift command expects a directory structure like:

    <path>/
      project1/
        state/           # Terraform state JSON files (terraform show -json)
        truth/           # osc list output files
          servers.json   # Output of: osc list servers -p project1 -r -o json
          secgrps.json   # Output of: osc list secgrps -p project1 -r -o json
      project2/
        state/
        truth/
      ...

Use subcommands to check for drift or generate truth files.`,
}

func init() {
	rootCmd.AddCommand(driftCmd)
}
