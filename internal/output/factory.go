package output

import (
	"fmt"
	"io"
	"strings"
)

// Format represents the supported output formats
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatCSV   Format = "csv"
)

// ErrInvalidFormat is returned when an unsupported format is specified
type ErrInvalidFormat struct {
	Format string
	Valid  []string
}

func (e *ErrInvalidFormat) Error() string {
	return fmt.Sprintf(
		"unsupported output format %q (valid formats: %s)",
		e.Format,
		strings.Join(e.Valid, ", "),
	)
}

// GetValidFormats returns a list of supported format strings
func GetValidFormats() []string {
	return []string{
		string(FormatTable),
		string(FormatJSON),
		string(FormatCSV),
	}
}

// NewFormatter creates a new formatter based on the specified format
func NewFormatter(format string, w io.Writer) (Formatter, error) {
	if !ValidateFormat(format) {
		return nil, &ErrInvalidFormat{
			Format: format,
			Valid:  GetValidFormats(),
		}
	}

	switch Format(format) {
	case FormatTable:
		return NewTableFormatter(w), nil
	case FormatJSON:
		return NewJSONFormatter(w), nil
	case FormatCSV:
		return NewCSVFormatter(w), nil
	default:
		// This should never happen due to ValidateFormat check
		return nil, fmt.Errorf("internal error: unhandled format %q", format)
	}
}

// ValidateFormat checks if the given format is supported
func ValidateFormat(format string) bool {
	switch Format(format) {
	case FormatTable, FormatJSON, FormatCSV:
		return true
	default:
		return false
	}
}

// FormatHelp returns a help string describing the available formats
func FormatHelp() string {
	return `Available output formats:
  table    Output in human-readable table format (default)
  json     Output in JSON format with metadata
  csv      Output in CSV format with headers`
}
