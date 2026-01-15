package drift

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// OscOutput represents the JSON output structure from osc list commands
// This matches the structure from internal/output/json.go
type OscOutput struct {
	Metadata *OscMetadata `json:"metadata,omitempty"`
	Headers  []string     `json:"headers"`
	Data     []OscRow     `json:"data"`
}

// OscMetadata contains metadata about the output
type OscMetadata struct {
	Filtering *OscFiltering `json:"filtering,omitempty"`
}

// OscFiltering contains information about project filtering
type OscFiltering struct {
	FilteredProjectCount int      `json:"filtered_project_count"`
	MatchedProjects      []string `json:"matched_projects"`
}

// OscRow represents a row of data from osc output
type OscRow struct {
	Type           string            `json:"type,omitempty"`
	Fields         map[string]string `json:"fields"`
	SecurityGroups []string          `json:"security_groups,omitempty"`
	RuleFields     *OscRuleFields    `json:"rule_fields,omitempty"`
}

// OscRuleFields contains security group rule specific fields
type OscRuleFields struct {
	Direction string `json:"direction,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	PortRange string `json:"port_range,omitempty"`
	RemoteIP  string `json:"remote_ip,omitempty"`
}

// ParseOscOutput parses osc JSON output from a reader
func ParseOscOutput(r io.Reader) (*OscOutput, error) {
	var output OscOutput
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&output); err != nil {
		return nil, fmt.Errorf("failed to parse osc output: %w", err)
	}
	return &output, nil
}

// ParseOscOutputFile parses osc JSON output from a file
func ParseOscOutputFile(path string) (*OscOutput, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open osc output file: %w", err)
	}
	defer f.Close()
	return ParseOscOutput(f)
}

// ExtractResourcesFromOsc extracts unified Resources from osc output
func ExtractResourcesFromOsc(output *OscOutput, projectName string) []Resource {
	if output == nil {
		return nil
	}

	var resources []Resource

	// Determine what type of data this is based on headers or row types
	for _, row := range output.Data {
		switch row.Type {
		case "security-group":
			if res := extractOscSecurityGroup(row, projectName); res != nil {
				resources = append(resources, *res)
			}
		case "security-group-rule":
			if res := extractOscSecurityGroupRule(row, projectName); res != nil {
				resources = append(resources, *res)
			}
		default:
			// This is likely a server row (no type field or type="server")
			if res := extractOscServer(row, projectName); res != nil {
				resources = append(resources, *res)
			}
		}
	}

	return resources
}

// extractOscServer extracts a server resource from osc row
func extractOscServer(row OscRow, projectName string) *Resource {
	// Try multiple field name variations (old format and new format)
	id := getOscField(row.Fields, "Server ID", "server_id", "id")
	name := getOscField(row.Fields, "Server Name", "server_name", "name")

	if id == "" {
		return nil
	}

	// Use projectName from parameter, or try to get from row
	project := projectName
	if project == "" {
		project = getOscField(row.Fields, "Project Name", "project_name", "project")
	}

	props := make(map[string]any)
	props["ip_address"] = getOscField(row.Fields, "IPv4 Address", "ipv4_address", "ip_address")

	return &Resource{
		ID:             id,
		Name:           name,
		Type:           ResourceTypeServer,
		ProjectName:    project,
		SecurityGroups: row.SecurityGroups,
		Properties:     props,
	}
}

// extractOscSecurityGroup extracts a security group resource from osc row
func extractOscSecurityGroup(row OscRow, projectName string) *Resource {
	id := getOscField(row.Fields, "ID", "id")
	name := getOscField(row.Fields, "Name", "name")

	if id == "" {
		return nil
	}

	project := projectName
	if project == "" {
		project = getOscField(row.Fields, "Project Name", "project_name", "project")
	}

	return &Resource{
		ID:          id,
		Name:        name,
		Type:        ResourceTypeSecurityGroup,
		ProjectName: project,
	}
}

// extractOscSecurityGroupRule extracts a security group rule resource from osc row
func extractOscSecurityGroupRule(row OscRow, projectName string) *Resource {
	id := getOscField(row.Fields, "ID", "id")
	parentID := getOscField(row.Fields, "Parent ID", "parent_id")
	parentName := getOscField(row.Fields, "Parent Name", "parent_name", "Name")

	if id == "" {
		return nil
	}

	project := projectName
	if project == "" {
		project = getOscField(row.Fields, "Project Name", "project_name", "project")
	}

	props := make(map[string]any)
	if row.RuleFields != nil {
		props["direction"] = row.RuleFields.Direction
		props["protocol"] = row.RuleFields.Protocol
		props["port_range"] = row.RuleFields.PortRange
		props["remote_ip"] = row.RuleFields.RemoteIP
	}

	return &Resource{
		ID:          id,
		Name:        "",
		Type:        ResourceTypeSecurityGroupRule,
		ProjectName: project,
		ParentID:    parentID,
		ParentName:  parentName,
		Properties:  props,
	}
}

// LoadTruthFromDir loads and merges all osc JSON files from a directory
func LoadTruthFromDir(dirPath, projectName string) ([]Resource, error) {
	var allResources []Resource

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read truth directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .json files
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		output, err := ParseOscOutputFile(filePath)
		if err != nil {
			// Log warning but continue with other files
			fmt.Printf("Warning: failed to parse %s: %v\n", filePath, err)
			continue
		}

		resources := ExtractResourcesFromOsc(output, projectName)
		allResources = append(allResources, resources...)
	}

	return allResources, nil
}

// getOscField tries multiple field names and returns the first non-empty value
func getOscField(fields map[string]string, keys ...string) string {
	for _, key := range keys {
		if v, ok := fields[key]; ok && v != "" && v != "n/a" {
			return v
		}
	}
	return ""
}
