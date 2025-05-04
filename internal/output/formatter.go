package output

import (
	"io"
)

// OutputData represents the structured data to be formatted
type OutputData struct {
	Headers []string
	Rows    [][]string
	// Optional metadata for special cases like filtering results
	FilteredProjectCount int
	MatchedProjects      []string
	HasFiltering         bool
}

// Formatter defines the interface for different output formats
type Formatter interface {
	// Format writes the formatted output to the writer
	Format(data *OutputData) error
}

// BaseFormatter provides common functionality for formatters
type BaseFormatter struct {
	Writer io.Writer
}

// NewOutputData creates a new OutputData instance with the given headers and rows
func NewOutputData(headers []string, rows [][]string) *OutputData {
	return &OutputData{
		Headers: headers,
		Rows:    rows,
	}
}

// WithFilterInfo adds filtering information to the output data
func (d *OutputData) WithFilterInfo(matchedProjects []string) *OutputData {
	d.HasFiltering = true
	d.MatchedProjects = matchedProjects
	d.FilteredProjectCount = len(matchedProjects)
	return d
}
