/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/marcdicarlo/osc/internal/config"
	"github.com/marcdicarlo/osc/internal/db"
	"github.com/marcdicarlo/osc/internal/filter"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// secgrpsCmd represents the secgrps command
var (
	rules      bool
	secgrpsCmd = &cobra.Command{
		Use:   "secgrps",
		Short: "List all Openstack security groups for a project",
		Long: `List all Openstack security groups for a project.

Examples:

# list all security groups for a project
osc list secgrps -p "prod-app1"

# list all security groups and rules for a project
osc list secgrps -p "prod-app1" -r
`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load("config.yaml")
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}
			db, err := db.InitDB(cfg)
			if err != nil {
				log.Fatalf("Failed to init db: %v", err)
			}
			defer db.Close()
			Secgrps(db, cfg)
		},
	}
)

func init() {
	listCmd.AddCommand(secgrpsCmd)
	secgrpsCmd.Flags().BoolVarP(&rules, "rules", "r", false, "Show rules for each security group")
	secgrpsCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Filter security groups by project name (shows projects containing this string)")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// secgrpsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// secgrpsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Secgrps reads and outputs security group data.
func Secgrps(db *sql.DB, cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()
	query := `SELECT s.secgrp_name, s.secgrp_id, s.project_id, p.project_name
	FROM ` + cfg.Tables.SecGrps + ` s
	JOIN ` + cfg.Tables.Projects + ` p USING (project_id)
	ORDER BY s.secgrp_name;`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Initialize table
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Security Group Name", "Security Group ID", "Project ID", "Project Name"})
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetTablePadding("\t")

	var data [][]string
	var secGroups = make(map[string]string) // map[security_group_id]security_group_name

	for rows.Next() {
		var name, id, pid, pname string
		if err := rows.Scan(&name, &id, &pid, &pname); err != nil {
			return err
		}
		data = append(data, []string{name, id, pid, pname})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Apply project filtering
	pf := filter.New(projectFilter, cfg)
	filteredData, matchedProjects := pf.MatchProjects(data, 3) // 3 is the index of project_name in our data

	if pf.GetActiveFilter() != "" {
		if len(matchedProjects) == 0 {
			fmt.Print(pf.FormatMatchedProjects(matchedProjects, "security groups"))
			return nil
		}
		fmt.Print(pf.FormatMatchedProjects(matchedProjects, "security groups"))
		fmt.Println()
	}

	// Build the map of security groups after filtering
	if rules {
		for _, row := range filteredData {
			secGroups[row[1]] = row[0] // Store security group ID -> name mapping only for filtered groups
		}
	}

	for _, row := range filteredData {
		table.Append(row)
	}

	table.Render()

	// If rules flag is set, display rules for each security group
	if rules && len(filteredData) > 0 {
		fmt.Println("\nSecurity Group Rules:")
		for sgID, sgName := range secGroups {
			rulesQuery := `SELECT direction, protocol, 
				CASE 
					WHEN port_range_min IS NULL AND port_range_max IS NULL THEN 'any'
					WHEN port_range_min = port_range_max THEN CAST(port_range_min AS TEXT)
					ELSE CAST(port_range_min AS TEXT) || '-' || CAST(port_range_max AS TEXT)
				END as port_range,
				remote_ip_prefix as cidr
			FROM ` + cfg.Tables.SecGrpRules + `
			WHERE secgrp_id = ?
			ORDER BY direction, protocol;`

			ruleRows, err := db.QueryContext(ctx, rulesQuery, sgID)
			if err != nil {
				return fmt.Errorf("failed to fetch rules for security group %s: %v", sgID, err)
			}

			hasRules := false
			fmt.Printf("\n%s (%s):\n", sgName, sgID)
			rulesTable := tablewriter.NewWriter(os.Stdout)
			rulesTable.SetHeader([]string{"Direction", "Protocol", "Port Range", "CIDR"})
			rulesTable.SetAutoWrapText(false)
			rulesTable.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
			rulesTable.SetAlignment(tablewriter.ALIGN_LEFT)
			rulesTable.SetCenterSeparator("")
			rulesTable.SetColumnSeparator("")
			rulesTable.SetRowSeparator("")
			rulesTable.SetHeaderLine(false)
			rulesTable.SetBorder(false)
			rulesTable.SetTablePadding("\t")
			rulesTable.SetNoWhiteSpace(true)

			// Use a cleanup function to ensure ruleRows is closed after we're done with it
			func() {
				defer ruleRows.Close()
				for ruleRows.Next() {
					hasRules = true
					var direction, protocol, portRange, cidr string
					if err := ruleRows.Scan(&direction, &protocol, &portRange, &cidr); err != nil {
						log.Printf("failed to scan rule for security group %s: %v", sgID, err)
						return
					}

					rulesTable.Append([]string{
						direction,
						protocol,
						portRange,
						cidr,
					})
				}

				if err := ruleRows.Err(); err != nil {
					log.Printf("error iterating rules for security group %s: %v", sgID, err)
					return
				}
			}()

			if !hasRules {
				fmt.Println("No rules found")
			} else {
				rulesTable.Render()
			}
		}
	}

	return nil
}
