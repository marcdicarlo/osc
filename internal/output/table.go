package output

import (
	"fmt"
	"io"

	"github.com/olekukonko/tablewriter"
)

// TableFormatter implements the Formatter interface for table output
type TableFormatter struct {
	BaseFormatter
}

// NewTableFormatter creates a new TableFormatter instance
func NewTableFormatter(w io.Writer) *TableFormatter {
	return &TableFormatter{
		BaseFormatter: BaseFormatter{Writer: w},
	}
}

// Format writes the data in table format
func (f *TableFormatter) Format(data *OutputData) error {
	// Handle filtering info if present
	if data.HasFiltering {
		if data.FilteredProjectCount == 0 {
			fmt.Fprintf(f.Writer, "No projects matched the filter criteria\n")
			return nil
		}
		fmt.Fprintf(f.Writer, "Found %d matching projects: %v\n",
			data.FilteredProjectCount,
			data.MatchedProjects)
		fmt.Fprintln(f.Writer)
	}

	// Create and configure table writer
	table := tablewriter.NewWriter(f.Writer)
	table.SetHeader(data.Headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetTablePadding("\t")

	// Add all rows
	for _, row := range data.Rows {
		table.Append(row)
	}

	// Render the table
	table.Render()
	return nil
}

// FormatSecurityGroupRules formats security group rules in a special table format
func (f *TableFormatter) FormatSecurityGroupRules(groupName, groupID string, rules [][]string) error {
	fmt.Fprintf(f.Writer, "\n%s (%s):\n", groupName, groupID)

	if len(rules) == 0 {
		fmt.Fprintln(f.Writer, "No rules found")
		return nil
	}

	table := tablewriter.NewWriter(f.Writer)
	table.SetHeader([]string{"Direction", "Protocol", "Port Range", "CIDR"})
	table.SetAutoWrapText(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)

	for _, rule := range rules {
		table.Append(rule)
	}

	table.Render()
	return nil
}
