package output

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
)

// JSONFormatter implements the Formatter interface for JSON output
type JSONFormatter struct {
	BaseFormatter
}

// JSONOutput represents the structure of the JSON output
type JSONOutput struct {
	Metadata *JSONMetadata `json:"metadata,omitempty"`
	Headers  []string      `json:"headers"`
	Data     []JSONRow     `json:"data"`
}

// JSONRow represents a row of data with type information
// Fields are now at top-level with normalized lowercase names
type JSONRow struct {
	// Common fields (normalized, lowercase)
	Type        string   `json:"type,omitempty"`
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name,omitempty"`
	ProjectName string   `json:"project_name,omitempty"`
	ProjectID   string   `json:"project_id,omitempty"`
	IPAddress   string   `json:"ip_address,omitempty"`

	// Server-specific fields
	SecurityGroups []string `json:"security_groups,omitempty"`

	// Security group rule-specific fields
	ParentID   string         `json:"parent_id,omitempty"`
	ParentName string         `json:"parent_name,omitempty"`
	RuleFields *JSONRuleFields `json:"rule_fields,omitempty"`

	// Legacy: keep fields map for backward compatibility during transition
	Fields map[string]string `json:"fields,omitempty"`
}

// JSONRuleFields contains security group rule specific fields
type JSONRuleFields struct {
	Direction   string `json:"direction,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	PortRange   string `json:"port_range,omitempty"`
	RemoteIP    string `json:"remote_ip,omitempty"`
	Ethertype   string `json:"ethertype,omitempty"`
	RemoteGroup string `json:"remote_group,omitempty"`
}

// JSONMetadata contains metadata about the output
type JSONMetadata struct {
	Filtering *JSONFiltering `json:"filtering,omitempty"`
}

// JSONFiltering contains information about project filtering
type JSONFiltering struct {
	FilteredProjectCount int      `json:"filtered_project_count"`
	MatchedProjects      []string `json:"matched_projects"`
}

// JSONSecurityGroupRules represents the structure for security group rules output
type JSONSecurityGroupRules struct {
	GroupName string     `json:"group_name"`
	GroupID   string     `json:"group_id"`
	Headers   []string   `json:"headers"`
	Rules     [][]string `json:"rules"`
}

// NewJSONFormatter creates a new JSONFormatter instance
func NewJSONFormatter(w io.Writer) *JSONFormatter {
	return &JSONFormatter{
		BaseFormatter: BaseFormatter{Writer: w},
	}
}

// normalizeHeaderName converts a header name to lowercase normalized form
func normalizeHeaderName(header string) string {
	// Map of old header names to new normalized names
	headerMap := map[string]string{
		"Server Name":     "name",
		"Server ID":       "id",
		"Project Name":    "project_name",
		"Project ID":      "project_id",
		"Parent ID":       "parent_id",
		"IPv4 Address":    "ip_address",
		"Security Groups": "security_groups",
		"Name":            "name",
		"ID":              "id",
		"Resource Type":   "type",
		"Direction":       "direction",
		"Protocol":        "protocol",
		"Port Range":      "port_range",
		"Remote IP":       "remote_ip",
		"Ethertype":       "ethertype",
		"Remote Group":    "remote_group",
	}

	if normalized, ok := headerMap[header]; ok {
		return normalized
	}
	// Convert to lowercase and replace spaces with underscores
	return strings.ToLower(strings.ReplaceAll(header, " ", "_"))
}

// Format writes the data in JSON format with normalized field names
func (f *JSONFormatter) Format(data *OutputData) error {
	if data == nil {
		return fmt.Errorf("nil output data provided")
	}

	if len(data.Headers) == 0 {
		return fmt.Errorf("no headers provided")
	}

	// Normalize headers (excluding Security Groups which becomes a separate field)
	normalizedHeaders := make([]string, 0, len(data.Headers))
	for _, h := range data.Headers {
		if h != "Security Groups" {
			normalizedHeaders = append(normalizedHeaders, normalizeHeaderName(h))
		}
	}

	output := JSONOutput{
		Headers: normalizedHeaders,
		Data:    make([]JSONRow, 0, len(data.Rows)),
	}

	// Add filtering metadata if present
	if data.HasFiltering {
		output.Metadata = &JSONMetadata{
			Filtering: &JSONFiltering{
				FilteredProjectCount: data.FilteredProjectCount,
				MatchedProjects:      data.MatchedProjects,
			},
		}
	}

	// Find important column indices
	headerIndices := make(map[string]int)
	for i, h := range data.Headers {
		headerIndices[h] = i
	}

	resourceTypeIndex := getIndex(headerIndices, "Resource Type")
	securityGroupsIndex := getIndex(headerIndices, "Security Groups")
	hasRules := resourceTypeIndex >= 0 && len(data.Headers) > 5

	// Convert rows to structured JSON format
	for rowIndex, row := range data.Rows {
		if len(row) < len(data.Headers) {
			log.Printf("Warning: Row %d has fewer fields (%d) than headers (%d)", rowIndex, len(row), len(data.Headers))
			continue
		}

		jsonRow := JSONRow{}

		// Determine the resource type first
		if resourceTypeIndex >= 0 && len(row) > resourceTypeIndex {
			jsonRow.Type = row[resourceTypeIndex]
		}

		// Extract common fields by checking headers
		jsonRow.ID = getFieldByHeader(row, headerIndices, "ID", "Server ID")
		jsonRow.Name = getFieldByHeader(row, headerIndices, "Name", "Server Name")
		jsonRow.ProjectName = getFieldByHeader(row, headerIndices, "Project Name")
		jsonRow.ProjectID = getFieldByHeader(row, headerIndices, "Project ID")
		jsonRow.ParentID = getFieldByHeader(row, headerIndices, "Parent ID")
		jsonRow.IPAddress = getFieldByHeader(row, headerIndices, "IPv4 Address")

		// Handle Security Groups column - convert to list
		if securityGroupsIndex >= 0 && len(row) > securityGroupsIndex {
			if row[securityGroupsIndex] != "" {
				groups := strings.Split(row[securityGroupsIndex], ", ")
				jsonRow.SecurityGroups = groups
			} else {
				jsonRow.SecurityGroups = []string{}
			}
		}

		// Handle security group rules
		if hasRules && jsonRow.Type == "security-group-rule" {
			// For rules, the "Name" field often contains the rule ID
			// and we need to extract parent info
			if jsonRow.Name == "" && jsonRow.ID != "" {
				// Rule ID might be in Name position, keep it as ID
			}

			// Extract rule fields by header index (more robust than hardcoded positions)
			directionIdx := getIndex(headerIndices, "Direction")
			protocolIdx := getIndex(headerIndices, "Protocol")
			portRangeIdx := getIndex(headerIndices, "Port Range")
			remoteIPIdx := getIndex(headerIndices, "Remote IP")
			ethertypeIdx := getIndex(headerIndices, "Ethertype")
			remoteGroupIdx := getIndex(headerIndices, "Remote Group")

			if directionIdx >= 0 && len(row) > directionIdx {
				jsonRow.RuleFields = &JSONRuleFields{
					Direction: getValueOrDefault(getRowValue(row, directionIdx), ""),
					Protocol:  getValueOrDefault(getRowValue(row, protocolIdx), ""),
					PortRange: getValueOrDefault(getRowValue(row, portRangeIdx), ""),
					RemoteIP:  getValueOrDefault(getRowValue(row, remoteIPIdx), ""),
				}

				// Full output includes ethertype and remote_group
				if ethertypeIdx >= 0 {
					jsonRow.RuleFields.Ethertype = getValueOrDefault(getRowValue(row, ethertypeIdx), "")
				}
				if remoteGroupIdx >= 0 {
					jsonRow.RuleFields.RemoteGroup = getValueOrDefault(getRowValue(row, remoteGroupIdx), "")
				}
			}
		}

		// Set type for servers if not already set
		if jsonRow.Type == "" && jsonRow.IPAddress != "" {
			jsonRow.Type = "server"
		}

		// Set type for security groups
		if jsonRow.Type == "" && resourceTypeIndex < 0 && jsonRow.ID != "" && jsonRow.Name != "" {
			// This is likely a server row if it has IP address, otherwise could be security group
			if jsonRow.IPAddress != "" {
				jsonRow.Type = "server"
			}
		}

		output.Data = append(output.Data, jsonRow)
	}

	// Use a buffer to catch any encoding errors
	encoder := json.NewEncoder(f.Writer)
	encoder.SetIndent("", "  ")  // Pretty print with 2 spaces
	encoder.SetEscapeHTML(false) // Don't escape HTML characters in the output

	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("error encoding JSON (data size: %d rows): %v", len(data.Rows), err)
	}

	return nil
}

// getIndex returns the index for a header name, or -1 if not found
func getIndex(headerIndices map[string]int, name string) int {
	if idx, ok := headerIndices[name]; ok {
		return idx
	}
	return -1
}

// getFieldByHeader gets a field value by trying multiple possible header names
func getFieldByHeader(row []string, headerIndices map[string]int, headerNames ...string) string {
	for _, name := range headerNames {
		if idx, ok := headerIndices[name]; ok && idx < len(row) {
			val := row[idx]
			if val != "" && val != "n/a" {
				return val
			}
		}
	}
	return ""
}

// getValueOrDefault returns the value if not empty, otherwise returns the default value
func getValueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// getRowValue safely gets a value from a row slice at the given index
func getRowValue(row []string, idx int) string {
	if idx >= 0 && idx < len(row) {
		return row[idx]
	}
	return ""
}

// FormatSecurityGroupRules formats security group rules in JSON format
func (f *JSONFormatter) FormatSecurityGroupRules(groupName, groupID string, rules [][]string) error {
	if groupName == "" || groupID == "" {
		return fmt.Errorf("group name and ID cannot be empty")
	}

	output := struct {
		GroupName string     `json:"group_name"`
		GroupID   string     `json:"group_id"`
		Headers   []string   `json:"headers"`
		Rules     [][]string `json:"rules"`
	}{
		GroupName: groupName,
		GroupID:   groupID,
		Headers:   []string{"direction", "protocol", "port_range", "remote_ip"},
		Rules:     rules,
	}

	encoder := json.NewEncoder(f.Writer)
	encoder.SetIndent("", "  ")  // Pretty print with 2 spaces
	encoder.SetEscapeHTML(false) // Don't escape HTML characters in the output

	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("error encoding security group rules JSON: %v", err)
	}

	return nil
}
