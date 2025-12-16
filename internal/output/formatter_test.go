package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestTableFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := NewTableFormatter(&buf)

	data := &OutputData{
		Headers: []string{"Name", "ID", "Project ID", "Project Name", "Resource Type"},
		Rows: [][]string{
			{"default", "sg-123", "proj-123", "prod-app1", "security-group"},
			{"rule-123", "sg-123", "proj-123", "prod-app1", "security-group-rule"},
		},
	}

	if err := f.Format(data); err != nil {
		t.Errorf("TableFormatter.Format() error = %v", err)
	}

	output := buf.String()
	outputLower := strings.ToLower(output)
	if !strings.Contains(outputLower, "name") || !strings.Contains(output, "default") {
		t.Errorf("TableFormatter output missing expected content. Got:\n%s", output)
	}
}

func TestJSONFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(&buf)

	data := &OutputData{
		Headers: []string{"Name", "ID", "Project ID", "Project Name", "Resource Type", "Direction", "Protocol", "Port Range", "Remote IP"},
		Rows: [][]string{
			{"default", "sg-123", "proj-123", "prod-app1", "security-group", "", "", "", ""},
			{"rule-123", "sg-123", "proj-123", "prod-app1", "security-group-rule", "ingress", "tcp", "22", "0.0.0.0/0"},
		},
		HasFiltering:         true,
		FilteredProjectCount: 1,
		MatchedProjects:      []string{"prod-app1"},
	}

	if err := f.Format(data); err != nil {
		t.Errorf("JSONFormatter.Format() error = %v", err)
	}

	var output JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Errorf("Failed to parse JSON output: %v", err)
	}

	// Verify metadata
	if output.Metadata == nil {
		t.Error("Expected metadata in JSON output")
	} else if output.Metadata.Filtering.FilteredProjectCount != 1 {
		t.Errorf("Expected FilteredProjectCount = 1, got %d", output.Metadata.Filtering.FilteredProjectCount)
	}

	// Verify data structure
	if len(output.Data) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(output.Data))
	}

	// Check security group
	sg := output.Data[0]
	if sg.Type != "security-group" {
		t.Errorf("Expected type security-group, got %s", sg.Type)
	}
	if sg.RuleFields != nil {
		t.Error("Security group should not have rule fields")
	}

	// Check security group rule
	rule := output.Data[1]
	if rule.Type != "security-group-rule" {
		t.Errorf("Expected type security-group-rule, got %s", rule.Type)
	}
	if rule.RuleFields == nil {
		t.Error("Security group rule should have rule fields")
	} else {
		if rule.RuleFields.Direction != "ingress" {
			t.Errorf("Expected direction ingress, got %s", rule.RuleFields.Direction)
		}
		if rule.RuleFields.Protocol != "tcp" {
			t.Errorf("Expected protocol tcp, got %s", rule.RuleFields.Protocol)
		}
		if rule.RuleFields.PortRange != "22" {
			t.Errorf("Expected port range 22, got %s", rule.RuleFields.PortRange)
		}
		if rule.RuleFields.RemoteIP != "0.0.0.0/0" {
			t.Errorf("Expected remote IP 0.0.0.0/0, got %s", rule.RuleFields.RemoteIP)
		}
	}
}

func TestCSVFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := NewCSVFormatter(&buf)

	data := &OutputData{
		Headers: []string{"Name", "ID", "Project ID", "Project Name", "Resource Type", "Direction", "Protocol", "Port Range", "Remote IP"},
		Rows: [][]string{
			{"default", "sg-123", "proj-123", "prod-app1", "security-group", "", "", "", ""},
			{"rule-123", "sg-123", "proj-123", "prod-app1", "security-group-rule", "ingress", "tcp", "22", "0.0.0.0/0"},
		},
		HasFiltering:         true,
		FilteredProjectCount: 1,
		MatchedProjects:      []string{"prod-app1"},
	}

	if err := f.Format(data); err != nil {
		t.Errorf("CSVFormatter.Format() error = %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Check filtering info comment
	if !strings.Contains(lines[0], "Found 1 matching projects") {
		t.Error("Missing filtering info in CSV output")
	}

	// Check headers
	headers := strings.Split(lines[1], ",")
	if len(headers) != 9 {
		t.Errorf("Expected 9 headers, got %d", len(headers))
	}

	// Check data rows
	if len(lines) != 4 { // Comment + headers + 2 data rows
		t.Errorf("Expected 4 lines, got %d", len(lines))
	}

	// Check security group rule data
	ruleFields := strings.Split(lines[3], ",")
	if ruleFields[5] != "ingress" || ruleFields[6] != "tcp" || ruleFields[7] != "22" || ruleFields[8] != "0.0.0.0/0" {
		t.Error("Security group rule fields not formatted correctly")
	}
}

func TestFormatterFactory(t *testing.T) {
	var buf bytes.Buffer

	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"table", "table", false},
		{"json", "json", false},
		{"csv", "csv", false},
		{"invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := NewFormatter(tt.format, &buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFormatter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && f == nil {
				t.Error("NewFormatter() returned nil formatter")
			}
		})
	}
}
