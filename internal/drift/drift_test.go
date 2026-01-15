package drift

import (
	"strings"
	"testing"
)

func TestParseTerraformState(t *testing.T) {
	// Sample Terraform state JSON (minimal)
	stateJSON := `{
		"format_version": "1.0",
		"terraform_version": "1.1.7",
		"values": {
			"root_module": {
				"resources": [
					{
						"address": "openstack_compute_instance_v2.test",
						"mode": "managed",
						"type": "openstack_compute_instance_v2",
						"name": "test",
						"values": {
							"id": "test-server-id-123",
							"name": "test-server",
							"access_ip_v4": "10.0.0.1",
							"flavor_name": "m6.medium",
							"security_groups": ["default", "web-servers"]
						}
					}
				]
			}
		}
	}`

	state, err := ParseTerraformState(strings.NewReader(stateJSON))
	if err != nil {
		t.Fatalf("Failed to parse Terraform state: %v", err)
	}

	if state.FormatVersion != "1.0" {
		t.Errorf("Expected format_version 1.0, got %s", state.FormatVersion)
	}

	resources := ExtractResourcesFromTerraform(state, "test-project")
	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}

	server := resources[0]
	if server.Type != ResourceTypeServer {
		t.Errorf("Expected server type, got %s", server.Type)
	}
	if server.ID != "test-server-id-123" {
		t.Errorf("Expected ID test-server-id-123, got %s", server.ID)
	}
	if server.Name != "test-server" {
		t.Errorf("Expected name test-server, got %s", server.Name)
	}
	if len(server.SecurityGroups) != 2 {
		t.Errorf("Expected 2 security groups, got %d", len(server.SecurityGroups))
	}
}

func TestParseOscOutput(t *testing.T) {
	// Sample osc JSON output (new normalized format with top-level fields)
	oscJSON := `{
		"headers": ["name", "id", "project_name", "ip_address"],
		"data": [
			{
				"type": "server",
				"id": "server-id-123",
				"name": "test-server",
				"project_name": "test-project",
				"ip_address": "10.0.0.1",
				"security_groups": ["default", "web-servers"]
			}
		]
	}`

	output, err := ParseOscOutput(strings.NewReader(oscJSON))
	if err != nil {
		t.Fatalf("Failed to parse osc output: %v", err)
	}

	if len(output.Data) != 1 {
		t.Fatalf("Expected 1 data row, got %d", len(output.Data))
	}

	// Verify the top-level fields are parsed correctly
	row := output.Data[0]
	if row.ID != "server-id-123" {
		t.Errorf("Expected ID server-id-123, got %s", row.ID)
	}
	if row.Name != "test-server" {
		t.Errorf("Expected name test-server, got %s", row.Name)
	}
	if row.ProjectName != "test-project" {
		t.Errorf("Expected project_name test-project, got %s", row.ProjectName)
	}
	if row.IPAddress != "10.0.0.1" {
		t.Errorf("Expected ip_address 10.0.0.1, got %s", row.IPAddress)
	}
}

func TestExtractResourcesFromNewJsonFormat(t *testing.T) {
	// Test extraction from the new normalized JSON format
	oscJSON := `{
		"headers": ["name", "id", "project_name", "ip_address"],
		"data": [
			{
				"type": "server",
				"id": "server-id-123",
				"name": "test-server",
				"project_name": "test-project",
				"ip_address": "10.0.0.1",
				"security_groups": ["default", "web-servers"]
			},
			{
				"type": "security-group",
				"id": "sg-id-456",
				"name": "web-servers",
				"project_name": "test-project"
			},
			{
				"type": "security-group-rule",
				"id": "rule-id-789",
				"parent_id": "sg-id-456",
				"parent_name": "web-servers",
				"project_name": "test-project"
			}
		]
	}`

	output, err := ParseOscOutput(strings.NewReader(oscJSON))
	if err != nil {
		t.Fatalf("Failed to parse osc output: %v", err)
	}

	resources := ExtractResourcesFromOsc(output, "test-project")

	if len(resources) != 3 {
		t.Fatalf("Expected 3 resources, got %d", len(resources))
	}

	// Check server
	server := resources[0]
	if server.Type != ResourceTypeServer {
		t.Errorf("Expected server type, got %s", server.Type)
	}
	if server.ID != "server-id-123" {
		t.Errorf("Expected server ID server-id-123, got %s", server.ID)
	}
	if server.Name != "test-server" {
		t.Errorf("Expected server name test-server, got %s", server.Name)
	}
	if len(server.SecurityGroups) != 2 {
		t.Errorf("Expected 2 security groups, got %d", len(server.SecurityGroups))
	}

	// Check security group
	sg := resources[1]
	if sg.Type != ResourceTypeSecurityGroup {
		t.Errorf("Expected security-group type, got %s", sg.Type)
	}
	if sg.ID != "sg-id-456" {
		t.Errorf("Expected sg ID sg-id-456, got %s", sg.ID)
	}
	if sg.Name != "web-servers" {
		t.Errorf("Expected sg name web-servers, got %s", sg.Name)
	}

	// Check security group rule
	rule := resources[2]
	if rule.Type != ResourceTypeSecurityGroupRule {
		t.Errorf("Expected security-group-rule type, got %s", rule.Type)
	}
	if rule.ID != "rule-id-789" {
		t.Errorf("Expected rule ID rule-id-789, got %s", rule.ID)
	}
	if rule.ParentID != "sg-id-456" {
		t.Errorf("Expected rule ParentID sg-id-456, got %s", rule.ParentID)
	}
}

func TestCompareResources(t *testing.T) {
	// Create test resources
	stateResources := []Resource{
		{
			ID:             "server-1",
			Name:           "web-server-1",
			Type:           ResourceTypeServer,
			ProjectName:    "project1",
			SecurityGroups: []string{"default", "web"},
		},
		{
			ID:          "server-2",
			Name:        "db-server-1",
			Type:        ResourceTypeServer,
			ProjectName: "project1",
		},
	}

	truthResources := []Resource{
		{
			ID:             "server-1",
			Name:           "web-server-1",
			Type:           ResourceTypeServer,
			ProjectName:    "project1",
			SecurityGroups: []string{"default", "web"},
		},
		{
			ID:          "server-3",
			Name:        "app-server-1",
			Type:        ResourceTypeServer,
			ProjectName: "project1",
		},
	}

	diffs := CompareResources(stateResources, truthResources)

	if len(diffs) != 2 {
		t.Fatalf("Expected 2 diffs, got %d", len(diffs))
	}

	// Check for missing in truth (server-2)
	var foundMissingInTruth, foundMissingInState bool
	for _, d := range diffs {
		if d.Status == StatusMissingInTruth && d.ResourceID == "server-2" {
			foundMissingInTruth = true
		}
		if d.Status == StatusMissingInState && d.ResourceID == "server-3" {
			foundMissingInState = true
		}
	}

	if !foundMissingInTruth {
		t.Error("Expected to find server-2 as missing_in_truth")
	}
	if !foundMissingInState {
		t.Error("Expected to find server-3 as missing_in_state")
	}
}

func TestCompareSecurityGroupChanges(t *testing.T) {
	stateResources := []Resource{
		{
			ID:             "server-1",
			Name:           "web-server-1",
			Type:           ResourceTypeServer,
			ProjectName:    "project1",
			SecurityGroups: []string{"default", "web"},
		},
	}

	truthResources := []Resource{
		{
			ID:             "server-1",
			Name:           "web-server-1",
			Type:           ResourceTypeServer,
			ProjectName:    "project1",
			SecurityGroups: []string{"default", "web", "monitoring"},
		},
	}

	diffs := CompareResources(stateResources, truthResources)

	if len(diffs) != 1 {
		t.Fatalf("Expected 1 diff for security group change, got %d", len(diffs))
	}

	if diffs[0].Status != StatusSecGroupChanged {
		t.Errorf("Expected status secgroups_changed, got %s", diffs[0].Status)
	}
}

func TestCountResources(t *testing.T) {
	resources := []Resource{
		{Type: ResourceTypeServer},
		{Type: ResourceTypeServer},
		{Type: ResourceTypeSecurityGroup},
		{Type: ResourceTypeSecurityGroupRule},
		{Type: ResourceTypeSecurityGroupRule},
		{Type: ResourceTypeSecurityGroupRule},
	}

	counts := CountResources(resources)

	if counts.Servers != 2 {
		t.Errorf("Expected 2 servers, got %d", counts.Servers)
	}
	if counts.SecurityGroups != 1 {
		t.Errorf("Expected 1 security group, got %d", counts.SecurityGroups)
	}
	if counts.SecurityGroupRules != 3 {
		t.Errorf("Expected 3 security group rules, got %d", counts.SecurityGroupRules)
	}
}

func TestDriftReport(t *testing.T) {
	report := NewDriftReport()

	project := ProjectDrift{
		ProjectName: "test-project",
		Drifts: []DiffResult{
			{
				ResourceType: ResourceTypeServer,
				ResourceID:   "server-1",
				Status:       StatusMissingInTruth,
			},
			{
				ResourceType: ResourceTypeSecurityGroup,
				ResourceID:   "sg-1",
				Status:       StatusMissingInState,
			},
		},
	}

	report.AddProject(project)

	if report.Summary.TotalProjects != 1 {
		t.Errorf("Expected 1 project, got %d", report.Summary.TotalProjects)
	}
	if report.Summary.TotalDrift != 2 {
		t.Errorf("Expected 2 total drift items, got %d", report.Summary.TotalDrift)
	}
	if !report.HasDrift() {
		t.Error("Expected HasDrift to return true")
	}
	if report.Summary.ByStatus[StatusMissingInTruth] != 1 {
		t.Errorf("Expected 1 missing_in_truth, got %d", report.Summary.ByStatus[StatusMissingInTruth])
	}
}
