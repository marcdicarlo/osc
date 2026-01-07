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

var showServerCmd = &cobra.Command{
	Use:   "server <server_name>",
	Short: "Show detailed information for a specific server",
	Long: `Show detailed information for a specific OpenStack server.

Shows server details including:
- Server ID, name, project, and IP address
- Attached security groups
- Attached volumes (table output only)

Examples:

# show server details (searches all projects with warning)
osc show server my-server

# show server in a specific project
osc show server my-server -p prod

# output in different formats
osc show server my-server -o json
osc show server my-server -o csv`,
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
		if err := ShowServer(database, cfg, args[0]); err != nil {
			log.Fatalf("Failed to show server: %v", err)
		}
	},
}

func init() {
	showCmd.AddCommand(showServerCmd)
	showServerCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Filter by project name")
}

// ServerDetail holds all information about a server
type ServerDetail struct {
	ServerID       string
	ServerName     string
	ProjectID      string
	ProjectName    string
	IPv4Addr       string
	Status         string
	ImageID        string
	ImageName      string
	FlavorID       string
	FlavorName     string
	SecurityGroups []SecurityGroupInfo
	Volumes        []VolumeInfo
}

// SecurityGroupInfo holds security group details
type SecurityGroupInfo struct {
	ID   string
	Name string
}

// VolumeInfo holds volume details
type VolumeInfo struct {
	ID         string
	Name       string
	SizeGB     int
	VolumeType string
	DevicePath string
}

// ShowServer displays detailed information about a specific server
func ShowServer(database *sql.DB, cfg *config.Config, serverName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	// Warning if no project filter specified
	if projectFilter == "" {
		fmt.Fprintln(os.Stderr, "Warning: No project specified (-p). Searching all projects...")
	}

	// Query for matching servers
	query := `SELECT s.server_id, s.server_name, s.project_id, p.project_name,
                     COALESCE(s.ipv4_addr, ''), COALESCE(s.status, ''),
                     COALESCE(s.image_id, ''), COALESCE(s.image_name, ''),
                     COALESCE(s.flavor_id, ''), COALESCE(s.flavor_name, '')
              FROM ` + cfg.Tables.Servers + ` s
              JOIN ` + cfg.Tables.Projects + ` p USING (project_id)
              WHERE s.server_name = ?`

	args := []interface{}{serverName}

	if projectFilter != "" {
		query += " AND LOWER(p.project_name) LIKE ?"
		args = append(args, "%"+strings.ToLower(projectFilter)+"%")
	}

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Collect matching servers
	var servers []ServerDetail
	for rows.Next() {
		var srv ServerDetail
		if err := rows.Scan(&srv.ServerID, &srv.ServerName, &srv.ProjectID, &srv.ProjectName,
			&srv.IPv4Addr, &srv.Status, &srv.ImageID, &srv.ImageName, &srv.FlavorID, &srv.FlavorName); err != nil {
			return err
		}
		servers = append(servers, srv)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(servers) == 0 {
		fmt.Printf("Server '%s' not found.\n", serverName)
		if projectFilter != "" {
			fmt.Printf("(searched in projects matching '%s')\n", projectFilter)
		}
		return nil
	}

	// If multiple matches, show them all with project info
	if len(servers) > 1 {
		fmt.Fprintf(os.Stderr, "Found %d servers matching '%s':\n\n", len(servers), serverName)
	}

	// Fetch security groups and volumes for each server
	for i := range servers {
		if err := fetchServerSecurityGroups(ctx, database, cfg, &servers[i]); err != nil {
			return err
		}
		if err := fetchServerVolumes(ctx, database, cfg, &servers[i]); err != nil {
			return err
		}
	}

	// Output based on format
	return outputServerDetails(servers)
}

func fetchServerSecurityGroups(ctx context.Context, database *sql.DB, cfg *config.Config, srv *ServerDetail) error {
	query := `SELECT sg.secgrp_id, sg.secgrp_name
              FROM ` + cfg.Tables.ServerSecGrps + ` ssg
              JOIN ` + cfg.Tables.SecGrps + ` sg ON ssg.secgrp_id = sg.secgrp_id
              WHERE ssg.server_id = ?`

	rows, err := database.QueryContext(ctx, query, srv.ServerID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var sg SecurityGroupInfo
		if err := rows.Scan(&sg.ID, &sg.Name); err != nil {
			return err
		}
		srv.SecurityGroups = append(srv.SecurityGroups, sg)
	}
	return rows.Err()
}

func fetchServerVolumes(ctx context.Context, database *sql.DB, cfg *config.Config, srv *ServerDetail) error {
	query := `SELECT v.volume_id, v.volume_name, v.size_gb, COALESCE(v.volume_type, ''), sv.device_path
              FROM ` + cfg.Tables.ServerVolumes + ` sv
              JOIN ` + cfg.Tables.Volumes + ` v ON sv.volume_id = v.volume_id
              WHERE sv.server_id = ?`

	rows, err := database.QueryContext(ctx, query, srv.ServerID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var vol VolumeInfo
		if err := rows.Scan(&vol.ID, &vol.Name, &vol.SizeGB, &vol.VolumeType, &vol.DevicePath); err != nil {
			return err
		}
		srv.Volumes = append(srv.Volumes, vol)
	}
	return rows.Err()
}

func outputServerDetails(servers []ServerDetail) error {
	switch outputFormat {
	case "json":
		return outputServerJSON(servers)
	case "csv":
		return outputServerCSV(servers)
	default:
		return outputServerTable(servers)
	}
}

// ServerJSON is the JSON output structure for a server
type ServerJSON struct {
	Server         string   `json:"server"`
	ServerID       string   `json:"server_id"`
	Status         string   `json:"status"`
	ProjectID      string   `json:"project_id"`
	ProjectName    string   `json:"project_name"`
	IPv4Addr       string   `json:"ipv4_addr"`
	ImageID        string   `json:"image_id"`
	ImageName      string   `json:"image_name"`
	FlavorID       string   `json:"flavor_id"`
	FlavorName     string   `json:"flavor_name"`
	SecurityGroups []string `json:"security_groups"`
}

func outputServerJSON(servers []ServerDetail) error {
	var output []ServerJSON
	for _, srv := range servers {
		sj := ServerJSON{
			Server:         srv.ServerName,
			ServerID:       srv.ServerID,
			Status:         srv.Status,
			ProjectID:      srv.ProjectID,
			ProjectName:    srv.ProjectName,
			IPv4Addr:       srv.IPv4Addr,
			ImageID:        srv.ImageID,
			ImageName:      srv.ImageName,
			FlavorID:       srv.FlavorID,
			FlavorName:     srv.FlavorName,
			SecurityGroups: make([]string, 0, len(srv.SecurityGroups)),
		}
		for _, sg := range srv.SecurityGroups {
			sj.SecurityGroups = append(sj.SecurityGroups, fmt.Sprintf("%s (%s)", sg.ID, sg.Name))
		}
		output = append(output, sj)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputServerCSV(servers []ServerDetail) error {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"server", "server_id", "status", "project_id", "project_name",
		"ipv4_addr", "image_id", "image_name", "flavor_id", "flavor_name", "security_groups"}); err != nil {
		return err
	}

	for _, srv := range servers {
		var sgList []string
		for _, sg := range srv.SecurityGroups {
			sgList = append(sgList, fmt.Sprintf("%s (%s)", sg.ID, sg.Name))
		}
		if err := writer.Write([]string{
			srv.ServerName,
			srv.ServerID,
			srv.Status,
			srv.ProjectID,
			srv.ProjectName,
			srv.IPv4Addr,
			srv.ImageID,
			srv.ImageName,
			srv.FlavorID,
			srv.FlavorName,
			strings.Join(sgList, ", "),
		}); err != nil {
			return err
		}
	}
	return nil
}

func outputServerTable(servers []ServerDetail) error {
	for i, srv := range servers {
		if i > 0 {
			fmt.Println() // Separator between multiple servers
		}
		fmt.Printf("Server: %s\n", srv.ServerName)
		fmt.Printf("  ID:           %s\n", srv.ServerID)
		fmt.Printf("  Status:       %s\n", srv.Status)
		fmt.Printf("  Project:      %s (%s)\n", srv.ProjectName, srv.ProjectID)
		fmt.Printf("  IPv4 Address: %s\n", srv.IPv4Addr)

		// Image info
		if srv.ImageID != "" || srv.ImageName != "" {
			if srv.ImageName != "" && srv.ImageID != "" {
				fmt.Printf("  Image:        %s (%s)\n", srv.ImageName, srv.ImageID)
			} else if srv.ImageName != "" {
				fmt.Printf("  Image:        %s\n", srv.ImageName)
			} else {
				fmt.Printf("  Image:        %s\n", srv.ImageID)
			}
		}

		// Flavor info
		if srv.FlavorID != "" || srv.FlavorName != "" {
			if srv.FlavorName != "" && srv.FlavorID != "" {
				fmt.Printf("  Flavor:       %s (%s)\n", srv.FlavorName, srv.FlavorID)
			} else if srv.FlavorName != "" {
				fmt.Printf("  Flavor:       %s\n", srv.FlavorName)
			} else {
				fmt.Printf("  Flavor:       %s\n", srv.FlavorID)
			}
		}

		fmt.Printf("\n  Security Groups:\n")
		if len(srv.SecurityGroups) == 0 {
			fmt.Printf("    (none)\n")
		} else {
			for _, sg := range srv.SecurityGroups {
				fmt.Printf("    - %s (%s)\n", sg.Name, sg.ID)
			}
		}

		fmt.Printf("\n  Volumes:\n")
		if len(srv.Volumes) == 0 {
			fmt.Printf("    (none)\n")
		} else {
			for _, vol := range srv.Volumes {
				volType := vol.VolumeType
				if volType == "" {
					volType = "default"
				}
				if vol.DevicePath != "" {
					fmt.Printf("    - %s (%s): %dGB %s @ %s\n",
						vol.Name, vol.ID, vol.SizeGB, volType, vol.DevicePath)
				} else {
					fmt.Printf("    - %s (%s): %dGB %s\n",
						vol.Name, vol.ID, vol.SizeGB, volType)
				}
			}
		}
	}
	return nil
}
