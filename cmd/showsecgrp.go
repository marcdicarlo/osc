package cmd

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/marcdicarlo/osc/internal/config"
	"github.com/marcdicarlo/osc/internal/db"
	"github.com/spf13/cobra"
)

var showSecGrpCmd = &cobra.Command{
	Use:   "secgrp <secgrp_name>",
	Short: "Show detailed information for a specific security group",
	Long: `Show detailed information for a specific OpenStack security group.

Shows security group details including:
- Security group ID, name, and project
- All rules (ingress and egress)
- Servers using this security group

Examples:

# show security group details (searches all projects with warning)
osc show secgrp web-servers

# show security group in a specific project
osc show secgrp web-servers -p prod

# output in different formats
osc show secgrp web-servers -o json
osc show secgrp web-servers -o csv`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		database, err := db.InitDB(cfg)
		if err != nil {
			log.Fatalf("Failed to init db: %v", err)
		}
		defer database.Close()
		if err := ShowSecGrp(database, cfg, args[0]); err != nil {
			log.Fatalf("Failed to show security group: %v", err)
		}
	},
}

func init() {
	showCmd.AddCommand(showSecGrpCmd)
	showSecGrpCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Filter by project name")
}

// SecGrpDetail holds all information about a security group
type SecGrpDetail struct {
	SecGrpID    string
	SecGrpName  string
	ProjectID   string
	ProjectName string
	Rules       []RuleInfo
	Servers     []ServerInfo
}

// RuleInfo holds security group rule details
type RuleInfo struct {
	RuleID          string
	Direction       string
	EtherType       string
	Protocol        string
	PortRangeMin    *int
	PortRangeMax    *int
	RemoteIPPrefix  string
	RemoteGroupID   string
	RemoteGroupName string
}

// ServerInfo holds server details for servers using this security group
type ServerInfo struct {
	ID   string
	Name string
}

// ShowSecGrp displays detailed information about a specific security group
func ShowSecGrp(database *sql.DB, cfg *config.Config, secgrpName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Warning if no project filter specified
	if projectFilter == "" {
		fmt.Fprintln(os.Stderr, "Warning: No project specified (-p). Searching all projects...")
	}

	// Query for matching security groups
	query := `SELECT sg.secgrp_id, sg.secgrp_name, sg.project_id, p.project_name
              FROM ` + cfg.Tables.SecGrps + ` sg
              JOIN ` + cfg.Tables.Projects + ` p USING (project_id)
              WHERE sg.secgrp_name = ?`

	args := []interface{}{secgrpName}

	if projectFilter != "" {
		query += " AND LOWER(p.project_name) LIKE ?"
		args = append(args, "%"+strings.ToLower(projectFilter)+"%")
	}

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Collect matching security groups
	var secgrps []SecGrpDetail
	for rows.Next() {
		var sg SecGrpDetail
		if err := rows.Scan(&sg.SecGrpID, &sg.SecGrpName, &sg.ProjectID, &sg.ProjectName); err != nil {
			return err
		}
		secgrps = append(secgrps, sg)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(secgrps) == 0 {
		fmt.Printf("Security group '%s' not found.\n", secgrpName)
		if projectFilter != "" {
			fmt.Printf("(searched in projects matching '%s')\n", projectFilter)
		}
		return nil
	}

	// If multiple matches, show them all with project info
	if len(secgrps) > 1 {
		fmt.Fprintf(os.Stderr, "Found %d security groups matching '%s':\n\n", len(secgrps), secgrpName)
	}

	// Fetch rules and servers for each security group
	for i := range secgrps {
		if err := fetchSecGrpRules(ctx, database, cfg, &secgrps[i]); err != nil {
			return err
		}
		if err := fetchSecGrpServers(ctx, database, cfg, &secgrps[i]); err != nil {
			return err
		}
	}

	// Output based on format
	return outputSecGrpDetails(secgrps)
}

func fetchSecGrpRules(ctx context.Context, database *sql.DB, cfg *config.Config, sg *SecGrpDetail) error {
	query := `SELECT r.rule_id, r.direction, r.ethertype,
                     COALESCE(r.protocol, 'any') as protocol,
                     r.port_range_min, r.port_range_max,
                     COALESCE(r.remote_ip_prefix, '') as remote_ip_prefix,
                     COALESCE(r.remote_group_id, '') as remote_group_id,
                     COALESCE(sg_remote.secgrp_name, '') as remote_group_name
              FROM ` + cfg.Tables.SecGrpRules + ` r
              LEFT JOIN ` + cfg.Tables.SecGrps + ` sg_remote ON r.remote_group_id = sg_remote.secgrp_id
              WHERE r.secgrp_id = ?
              ORDER BY r.direction, r.protocol, r.port_range_min`

	rows, err := database.QueryContext(ctx, query, sg.SecGrpID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rule RuleInfo
		var portMin, portMax sql.NullInt64
		if err := rows.Scan(&rule.RuleID, &rule.Direction, &rule.EtherType, &rule.Protocol,
			&portMin, &portMax, &rule.RemoteIPPrefix, &rule.RemoteGroupID, &rule.RemoteGroupName); err != nil {
			return err
		}
		if portMin.Valid {
			min := int(portMin.Int64)
			rule.PortRangeMin = &min
		}
		if portMax.Valid {
			max := int(portMax.Int64)
			rule.PortRangeMax = &max
		}
		sg.Rules = append(sg.Rules, rule)
	}
	return rows.Err()
}

func fetchSecGrpServers(ctx context.Context, database *sql.DB, cfg *config.Config, sg *SecGrpDetail) error {
	query := `SELECT s.server_id, s.server_name
              FROM ` + cfg.Tables.ServerSecGrps + ` ssg
              JOIN ` + cfg.Tables.Servers + ` s ON ssg.server_id = s.server_id
              WHERE ssg.secgrp_id = ?
              ORDER BY s.server_name`

	rows, err := database.QueryContext(ctx, query, sg.SecGrpID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var srv ServerInfo
		if err := rows.Scan(&srv.ID, &srv.Name); err != nil {
			return err
		}
		sg.Servers = append(sg.Servers, srv)
	}
	return rows.Err()
}

func outputSecGrpDetails(secgrps []SecGrpDetail) error {
	switch outputFormat {
	case "json":
		return outputSecGrpJSON(secgrps)
	case "csv":
		return outputSecGrpCSV(secgrps)
	default:
		return outputSecGrpTable(secgrps)
	}
}

// SecGrpJSON is the JSON output structure for a security group
type SecGrpJSON struct {
	SecGrpName  string        `json:"secgrp_name"`
	SecGrpID    string        `json:"secgrp_id"`
	ProjectID   string        `json:"project_id"`
	ProjectName string        `json:"project_name"`
	Rules       []RuleJSON    `json:"rules"`
	Servers     []string      `json:"servers"`
}

// RuleJSON is the JSON output structure for a security group rule
type RuleJSON struct {
	RuleID          string `json:"rule_id"`
	Direction       string `json:"direction"`
	EtherType       string `json:"ethertype"`
	Protocol        string `json:"protocol"`
	PortRangeMin    *int   `json:"port_range_min"`
	PortRangeMax    *int   `json:"port_range_max"`
	RemoteIPPrefix  string `json:"remote_ip_prefix,omitempty"`
	RemoteGroupID   string `json:"remote_group_id,omitempty"`
	RemoteGroupName string `json:"remote_group_name,omitempty"`
}

func outputSecGrpJSON(secgrps []SecGrpDetail) error {
	var output []SecGrpJSON
	for _, sg := range secgrps {
		sj := SecGrpJSON{
			SecGrpName:  sg.SecGrpName,
			SecGrpID:    sg.SecGrpID,
			ProjectID:   sg.ProjectID,
			ProjectName: sg.ProjectName,
			Rules:       make([]RuleJSON, 0, len(sg.Rules)),
			Servers:     make([]string, 0, len(sg.Servers)),
		}
		for _, rule := range sg.Rules {
			rj := RuleJSON{
				RuleID:          rule.RuleID,
				Direction:       rule.Direction,
				EtherType:       rule.EtherType,
				Protocol:        rule.Protocol,
				PortRangeMin:    rule.PortRangeMin,
				PortRangeMax:    rule.PortRangeMax,
				RemoteIPPrefix:  rule.RemoteIPPrefix,
				RemoteGroupID:   rule.RemoteGroupID,
				RemoteGroupName: rule.RemoteGroupName,
			}
			sj.Rules = append(sj.Rules, rj)
		}
		for _, srv := range sg.Servers {
			sj.Servers = append(sj.Servers, fmt.Sprintf("%s (%s)", srv.ID, srv.Name))
		}
		output = append(output, sj)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputSecGrpCSV(secgrps []SecGrpDetail) error {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"secgrp_name", "secgrp_id", "project_id", "project_name", "rules", "servers"}); err != nil {
		return err
	}

	for _, sg := range secgrps {
		// Serialize rules to JSON for CSV
		rulesJSON := "[]"
		if len(sg.Rules) > 0 {
			rulesBytes, err := json.Marshal(sg.Rules)
			if err == nil {
				rulesJSON = string(rulesBytes)
			}
		}

		// Format servers list
		var serverList []string
		for _, srv := range sg.Servers {
			serverList = append(serverList, fmt.Sprintf("%s (%s)", srv.ID, srv.Name))
		}

		if err := writer.Write([]string{
			sg.SecGrpName,
			sg.SecGrpID,
			sg.ProjectID,
			sg.ProjectName,
			rulesJSON,
			strings.Join(serverList, ", "),
		}); err != nil {
			return err
		}
	}
	return nil
}

func outputSecGrpTable(secgrps []SecGrpDetail) error {
	for i, sg := range secgrps {
		if i > 0 {
			fmt.Println() // Separator between multiple security groups
		}
		fmt.Printf("Security Group: %s\n", sg.SecGrpName)
		fmt.Printf("  ID:      %s\n", sg.SecGrpID)
		fmt.Printf("  Project: %s (%s)\n", sg.ProjectName, sg.ProjectID)

		fmt.Printf("\n  Rules:\n")
		if len(sg.Rules) == 0 {
			fmt.Printf("    (none)\n")
		} else {
			for _, rule := range sg.Rules {
				// Format port range
				portStr := "any"
				if rule.PortRangeMin != nil && rule.PortRangeMax != nil {
					if *rule.PortRangeMin == *rule.PortRangeMax {
						portStr = fmt.Sprintf("%d", *rule.PortRangeMin)
					} else {
						portStr = fmt.Sprintf("%d-%d", *rule.PortRangeMin, *rule.PortRangeMax)
					}
				}

				// Format remote source/destination
				remote := ""
				if rule.RemoteGroupID != "" {
					if rule.RemoteGroupName != "" {
						remote = fmt.Sprintf("%s (%s)", rule.RemoteGroupID, rule.RemoteGroupName)
					} else {
						remote = rule.RemoteGroupID
					}
				} else if rule.RemoteIPPrefix != "" {
					remote = rule.RemoteIPPrefix
				} else {
					remote = "any"
				}

				// Direction determines from/to
				direction := strings.ToUpper(rule.Direction)
				preposition := "from"
				if rule.Direction == "egress" {
					preposition = "to"
				}

				fmt.Printf("    %-7s %-4s %-10s %s %s\n",
					direction, rule.Protocol, portStr, preposition, remote)
			}
		}

		fmt.Printf("\n  Servers Using This Group:\n")
		if len(sg.Servers) == 0 {
			fmt.Printf("    (none)\n")
		} else {
			for _, srv := range sg.Servers {
				fmt.Printf("    - %s (%s)\n", srv.Name, srv.ID)
			}
		}
	}
	return nil
}
