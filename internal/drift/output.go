package drift

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// OutputFormat represents the output format type
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatCSV   OutputFormat = "csv"
)

// DriftFormatter formats drift reports
type DriftFormatter struct {
	Writer io.Writer
	Format OutputFormat
}

// NewDriftFormatter creates a new drift formatter
func NewDriftFormatter(w io.Writer, format string) *DriftFormatter {
	var f OutputFormat
	switch strings.ToLower(format) {
	case "json":
		f = FormatJSON
	case "csv":
		f = FormatCSV
	default:
		f = FormatTable
	}
	return &DriftFormatter{Writer: w, Format: f}
}

// FormatReport formats a drift report according to the formatter's format
func (f *DriftFormatter) FormatReport(report *DriftReport) error {
	switch f.Format {
	case FormatJSON:
		return f.formatJSON(report)
	case FormatCSV:
		return f.formatCSV(report)
	default:
		return f.formatTable(report)
	}
}

// formatTable formats the drift report as a table
func (f *DriftFormatter) formatTable(report *DriftReport) error {
	w := tabwriter.NewWriter(f.Writer, 0, 0, 2, ' ', 0)

	// Print header
	fmt.Fprintln(w, "PROJECT\tRESOURCE TYPE\tNAME\tID\tSTATUS\tDETAILS")
	fmt.Fprintln(w, "-------\t-------------\t----\t--\t------\t-------")

	// Print rows for each project
	for _, project := range report.Projects {
		for _, drift := range project.Drifts {
			name := drift.ResourceName
			if name == "" && drift.ParentSG != "" {
				name = fmt.Sprintf("(rule in %s)", drift.ParentSG)
			}
			if name == "" {
				name = "(unnamed)"
			}

			// Truncate ID for display
			id := truncateID(drift.ResourceID, 12)

			// Truncate details for table display
			details := truncateString(drift.Details, 50)

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				project.ProjectName,
				drift.ResourceType,
				name,
				id,
				drift.Status,
				details,
			)
		}
	}

	// Print summary
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Summary: %d projects, %d drift items\n",
		report.Summary.TotalProjects,
		report.Summary.TotalDrift,
	)

	if report.Summary.TotalDrift > 0 {
		fmt.Fprintf(w, "By status: ")
		var statusParts []string
		for status, count := range report.Summary.ByStatus {
			statusParts = append(statusParts, fmt.Sprintf("%s=%d", status, count))
		}
		fmt.Fprintln(w, strings.Join(statusParts, ", "))
	}

	return w.Flush()
}

// formatJSON formats the drift report as JSON
func (f *DriftFormatter) formatJSON(report *DriftReport) error {
	encoder := json.NewEncoder(f.Writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(report)
}

// formatCSV formats the drift report as CSV
func (f *DriftFormatter) formatCSV(report *DriftReport) error {
	w := csv.NewWriter(f.Writer)

	// Write header
	header := []string{"project", "resource_type", "name", "id", "parent_sg", "status", "details"}
	if err := w.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write rows
	for _, project := range report.Projects {
		for _, drift := range project.Drifts {
			row := []string{
				project.ProjectName,
				string(drift.ResourceType),
				drift.ResourceName,
				drift.ResourceID,
				drift.ParentSG,
				string(drift.Status),
				drift.Details,
			}
			if err := w.Write(row); err != nil {
				return fmt.Errorf("failed to write CSV row: %w", err)
			}
		}
	}

	w.Flush()
	return w.Error()
}

// truncateID truncates an ID for display, showing first n characters with ellipsis
func truncateID(id string, maxLen int) string {
	if len(id) <= maxLen {
		return id
	}
	return id[:maxLen-3] + "..."
}

// truncateString truncates a string for display
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// PrintNoDrift prints a message when no drift is detected
func (f *DriftFormatter) PrintNoDrift(projectCount int) {
	switch f.Format {
	case FormatJSON:
		report := NewDriftReport()
		report.Summary.TotalProjects = projectCount
		f.formatJSON(report)
	case FormatCSV:
		// For CSV, just print header with no rows
		w := csv.NewWriter(f.Writer)
		w.Write([]string{"project", "resource_type", "name", "id", "parent_sg", "status", "details"})
		w.Flush()
	default:
		fmt.Fprintf(f.Writer, "No drift detected across %d projects.\n", projectCount)
	}
}
