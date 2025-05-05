package cmd

import (
	"fmt"

	"github.com/marcdicarlo/osc/internal/version"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of osc",
	Long:  `Print the version number, git commit, and build date of the osc tool`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("osc", version.GetFullVersion())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
