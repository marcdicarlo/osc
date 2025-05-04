/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Used by multiple commands
	projectFilter string
	outputFormat  string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "osc",
	Short: "An OpenStack caching application",
	Long: `A small go application that caches some data from the openstack api and caches it locally for performance. For example:

Command options:
    list        Display one or more resouces
    sync        Synchronize one or more types of resouce data into the DB

Usage:
    osc <command> [flags]

Use "osc <command> --help" for more information about a given command.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.osc.yaml)")

	// Add global output format flag
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, or csv")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	// Add list command
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List OpenStack resources",
		Long:  `List OpenStack resources such as servers, security groups, etc.`,
	}
	rootCmd.AddCommand(listCmd)
	listCmd.PersistentFlags().StringVarP(&projectFilter, "project", "p", "", "Filter by project name (shows resources in projects containing this string)")
}
