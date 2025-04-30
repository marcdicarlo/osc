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

// serversCmd represents the servers command
var serversCmd = &cobra.Command{
	Use:   "servers",
	Short: "List all OpenStack servers",
	Long: `List all OpenStack servers.

Examples:

# list all openstack servers
osc list servers

# list servers in projects containing a string
osc list servers -p "prod"    # matches: prod-app1, prod-app2, production
osc list servers -p "eta"     # matches: hc_zeta_project, hc_eta_project, hc_beta_project
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
		Servers(db, cfg)
	},
}

func init() {
	listCmd.AddCommand(serversCmd)
	serversCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Filter servers by project name (shows projects containing this string)")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serversCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serversCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Servers reads and outputs server/project data.
func Servers(db *sql.DB, cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()
	query := `SELECT s.server_name, s.server_id, p.project_name, s.ipv4_addr
	FROM ` + cfg.Tables.Servers + ` s
	JOIN ` + cfg.Tables.Projects + ` p USING (project_id)
	ORDER BY s.server_name;`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Initialize table
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Server Name", "Server ID", "Project Name", "IPv4 Address"})

	// table := tablewriter.NewWriter(os.Stdout)
	// table.SetHeader([]string{"Server Name", "Server ID", "Project ID", "Project Name", "IPv4 Address"})
	// table.SetAutoWrapText(false)
	// table.SetAutoFormatHeaders(true)
	// table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	// table.SetAlignment(tablewriter.ALIGN_LEFT)
	// table.SetCenterSeparator("")
	// table.SetColumnSeparator("")
	// table.SetRowSeparator("")
	// table.SetHeaderLine(false)
	// table.SetBorder(false)
	// table.SetTablePadding("\t")
	// table.SetNoWhiteSpace(true)

	var data [][]string
	for rows.Next() {
		var name, id, pname, ipv4 string
		if err := rows.Scan(&name, &id, &pname, &ipv4); err != nil {
			return err
		}
		data = append(data, []string{name, id, pname, ipv4})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Apply project filtering
	pf := filter.New(projectFilter, cfg)
	filteredData, matchedProjects := pf.MatchProjects(data, 2) // 2 is the index of project_name in our data

	if pf.GetActiveFilter() != "" {
		if len(matchedProjects) == 0 {
			fmt.Print(pf.FormatMatchedProjects(matchedProjects, "servers"))
			return nil
		}
		fmt.Print(pf.FormatMatchedProjects(matchedProjects, "servers"))
		fmt.Println()
	}

	for _, row := range filteredData {
		table.Append(row)
	}

	table.Render()
	return nil
}
