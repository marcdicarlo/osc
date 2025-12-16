package output

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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
type JSONRow struct {
	Type       string            `json:"type,omitempty"`
	Fields     map[string]string `json:"fields"`
	RuleFields *JSONRuleFields   `json:"rule_fields,omitempty"`
}

// JSONRuleFields contains security group rule specific fields
type JSONRuleFields struct {
	Direction string `json:"direction,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	PortRange string `json:"port_range,omitempty"`
	RemoteIP  string `json:"remote_ip,omitempty"`
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

// Format writes the data in JSON format
func (f *JSONFormatter) Format(data *OutputData) error {
	if data == nil {
		return fmt.Errorf("nil output data provided")
	}

	if len(data.Headers) == 0 {
		return fmt.Errorf("no headers provided")
	}

	output := JSONOutput{
		Headers: data.Headers,
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

	// Convert rows to structured JSON format
	hasRules := len(data.Headers) > 5 // Check if we have rule fields
	for rowIndex, row := range data.Rows {
		if len(row) < len(data.Headers) {
			log.Printf("Warning: Row %d has fewer fields (%d) than headers (%d)", rowIndex, len(row), len(data.Headers))
			continue
		}

		jsonRow := JSONRow{
			Fields: make(map[string]string),
		}

		// Add basic fields with validation
		for i := 0; i < len(data.Headers) && i < len(row); i++ {
			if i < 5 { // Basic fields
				if row[i] == "" {
					// Use a placeholder for empty values
					jsonRow.Fields[data.Headers[i]] = "n/a"
				} else {
					jsonRow.Fields[data.Headers[i]] = row[i]
				}
			}
		}

		// Set the type from the Resource Type field if available
		if len(row) > 4 {
			jsonRow.Type = row[4] // Resource Type is always at index 4
		}

		// Add rule fields if present and this is a security-group-rule row
		if hasRules && len(row) > 8 && jsonRow.Type == "security-group-rule" {
			jsonRow.RuleFields = &JSONRuleFields{
				Direction: getValueOrDefault(row[5], "n/a"),
				Protocol:  getValueOrDefault(row[6], "n/a"),
				PortRange: getValueOrDefault(row[7], "n/a"),
				RemoteIP:  getValueOrDefault(row[8], "n/a"),
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

// getValueOrDefault returns the value if not empty, otherwise returns the default value
func getValueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
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
		Headers:   []string{"Direction", "Protocol", "Port Range", "CIDR"},
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
