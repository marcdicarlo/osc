package output

import (
	"encoding/csv"
	"fmt"
	"io"
)

// CSVFormatter implements the Formatter interface for CSV output
type CSVFormatter struct {
	BaseFormatter
}

// NewCSVFormatter creates a new CSVFormatter instance
func NewCSVFormatter(w io.Writer) *CSVFormatter {
	return &CSVFormatter{
		BaseFormatter: BaseFormatter{Writer: w},
	}
}

// Format writes the data in CSV format
func (f *CSVFormatter) Format(data *OutputData) error {
	writer := csv.NewWriter(f.Writer)
	defer writer.Flush()

	// Handle filtering info if present
	if data.HasFiltering {
		if data.FilteredProjectCount == 0 {
			fmt.Fprintf(f.Writer, "# No projects matched the filter criteria\n")
			return nil
		}
		fmt.Fprintf(f.Writer, "# Found %d matching projects: %v\n",
			data.FilteredProjectCount,
			data.MatchedProjects)
	}

	// Write headers
	if err := writer.Write(data.Headers); err != nil {
		return fmt.Errorf("error writing headers: %v", err)
	}

	// Write data rows
	for _, row := range data.Rows {
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("error writing row: %v", err)
		}
	}

	return nil
}

// FormatSecurityGroupRules formats security group rules in CSV format
func (f *CSVFormatter) FormatSecurityGroupRules(groupName, groupID string, rules [][]string) error {
	writer := csv.NewWriter(f.Writer)
	defer writer.Flush()

	// Write group info as a comment
	fmt.Fprintf(f.Writer, "# Security Group: %s (%s)\n", groupName, groupID)

	if len(rules) == 0 {
		fmt.Fprintln(f.Writer, "# No rules found")
		return nil
	}

	// Write headers
	headers := []string{"Direction", "Protocol", "Port Range", "CIDR"}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("error writing headers: %v", err)
	}

	// Write rules
	for _, rule := range rules {
		if err := writer.Write(rule); err != nil {
			return fmt.Errorf("error writing rule: %v", err)
		}
	}

	return nil
}
