package output

import (
	"encoding/json"
	"fmt"
	"io"
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
	Direction string `json:"direction"`
	Protocol  string `json:"protocol"`
	PortRange string `json:"port_range"`
	RemoteIP  string `json:"remote_ip"`
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
	for _, row := range data.Rows {
		jsonRow := JSONRow{
			Fields: make(map[string]string),
		}

		// Add basic fields
		for i := 0; i < 5; i++ { // First 5 fields are always present
			jsonRow.Fields[data.Headers[i]] = row[i]
		}

		// Set the type from the Resource Type field
		jsonRow.Type = row[4] // Resource Type is always at index 4

		// Add rule fields if present
		if hasRules && len(row) > 5 {
			jsonRow.RuleFields = &JSONRuleFields{
				Direction: row[5],
				Protocol:  row[6],
				PortRange: row[7],
				RemoteIP:  row[8],
			}
		}

		output.Data = append(output.Data, jsonRow)
	}

	encoder := json.NewEncoder(f.Writer)
	encoder.SetIndent("", "  ") // Pretty print with 2 spaces
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("error encoding JSON: %v", err)
	}

	return nil
}

// FormatSecurityGroupRules formats security group rules in JSON format
func (f *JSONFormatter) FormatSecurityGroupRules(groupName, groupID string, rules [][]string) error {
	output := JSONSecurityGroupRules{
		GroupName: groupName,
		GroupID:   groupID,
		Headers:   []string{"Direction", "Protocol", "Port Range", "CIDR"},
		Rules:     rules,
	}

	encoder := json.NewEncoder(f.Writer)
	encoder.SetIndent("", "  ") // Pretty print with 2 spaces
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("error encoding JSON: %v", err)
	}

	return nil
}
