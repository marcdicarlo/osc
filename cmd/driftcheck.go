/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/marcdicarlo/osc/internal/drift"
	"github.com/spf13/cobra"
)

var (
	// driftCheckPath is the path for the check subcommand
	driftCheckPath string
	// driftResourceFilter filters by resource type
	driftResourceFilter string
	// driftStatusFilter filters by drift status
	driftStatusFilter string
)

// driftCheckCmd represents the drift check command
var driftCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for drift in project directories",
	Long: `Compare Terraform state files with osc truth files to detect infrastructure drift.

This command scans all project directories in the specified path and compares
the Terraform state (from state/ subdirectory) with the OpenStack truth
(from truth/ subdirectory).

Example:
    osc drift check --path ./tmp
    osc drift check --path ./tmp -o json
    osc drift check --path ./tmp --resource servers
    osc drift check --path ./tmp --status missing_in_truth`,
	RunE: runDriftCheck,
}

func init() {
	driftCmd.AddCommand(driftCheckCmd)

	driftCheckCmd.Flags().StringVarP(&driftCheckPath, "path", "p", "", "Path to directory containing project folders (required)")
	driftCheckCmd.MarkFlagRequired("path")

	driftCheckCmd.Flags().StringVarP(&driftResourceFilter, "resource", "r", "all", "Filter by resource type: servers, secgrps, rules, all")
	driftCheckCmd.Flags().StringVarP(&driftStatusFilter, "status", "s", "all", "Filter by status: missing_in_truth, missing_in_state, name_changed, all")
}

func runDriftCheck(cmd *cobra.Command, args []string) error {
	// Process all projects
	report, err := drift.ProcessAllProjects(driftCheckPath)
	if err != nil {
		return fmt.Errorf("failed to process projects: %w", err)
	}

	// Apply filters
	report = filterReport(report, driftResourceFilter, driftStatusFilter)

	// Format and output
	formatter := drift.NewDriftFormatter(os.Stdout, outputFormat)

	if !report.HasDrift() {
		formatter.PrintNoDrift(report.Summary.TotalProjects)
		return nil
	}

	if err := formatter.FormatReport(report); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Exit with code 1 if drift was detected
	os.Exit(1)
	return nil
}

// filterReport filters the drift report by resource type and status
func filterReport(report *drift.DriftReport, resourceFilter, statusFilter string) *drift.DriftReport {
	if resourceFilter == "all" && statusFilter == "all" {
		return report
	}

	filtered := drift.NewDriftReport()

	for _, project := range report.Projects {
		var filteredDrifts []drift.DiffResult

		for _, d := range project.Drifts {
			// Apply resource filter
			if resourceFilter != "all" {
				match := false
				switch resourceFilter {
				case "servers":
					match = d.ResourceType == drift.ResourceTypeServer
				case "secgrps":
					match = d.ResourceType == drift.ResourceTypeSecurityGroup
				case "rules":
					match = d.ResourceType == drift.ResourceTypeSecurityGroupRule
				}
				if !match {
					continue
				}
			}

			// Apply status filter
			if statusFilter != "all" {
				match := false
				switch statusFilter {
				case "missing_in_truth":
					match = d.Status == drift.StatusMissingInTruth
				case "missing_in_state":
					match = d.Status == drift.StatusMissingInState
				case "name_changed":
					match = d.Status == drift.StatusNameChanged
				case "secgroups_changed":
					match = d.Status == drift.StatusSecGroupChanged
				case "rule_changed":
					match = d.Status == drift.StatusRuleChanged
				}
				if !match {
					continue
				}
			}

			filteredDrifts = append(filteredDrifts, d)
		}

		if len(filteredDrifts) > 0 {
			filtered.AddProject(drift.ProjectDrift{
				ProjectName: project.ProjectName,
				Drifts:      filteredDrifts,
				StateCount:  project.StateCount,
				TruthCount:  project.TruthCount,
			})
		}
	}

	return filtered
}
