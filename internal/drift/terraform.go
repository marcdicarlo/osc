package drift

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// TerraformState represents the top-level structure of terraform show -json output
type TerraformState struct {
	FormatVersion    string          `json:"format_version"`
	TerraformVersion string          `json:"terraform_version"`
	Values           *TerraformValues `json:"values"`
}

// TerraformValues contains the root module
type TerraformValues struct {
	RootModule *TerraformRootModule `json:"root_module"`
}

// TerraformRootModule contains resources and child modules
type TerraformRootModule struct {
	Resources    []TerraformResource    `json:"resources"`
	ChildModules []TerraformChildModule `json:"child_modules,omitempty"`
}

// TerraformChildModule represents a child module in Terraform state
type TerraformChildModule struct {
	Address      string                 `json:"address"`
	Resources    []TerraformResource    `json:"resources"`
	ChildModules []TerraformChildModule `json:"child_modules,omitempty"`
}

// TerraformResource represents a single resource in Terraform state
type TerraformResource struct {
	Address      string                 `json:"address"`
	Mode         string                 `json:"mode"`
	Type         string                 `json:"type"`
	Name         string                 `json:"name"`
	Index        any                    `json:"index,omitempty"`
	ProviderName string                 `json:"provider_name"`
	Values       map[string]any         `json:"values"`
}

// OpenStack resource type constants
const (
	TerraformTypeComputeInstance = "openstack_compute_instance_v2"
	TerraformTypeSecurityGroup   = "openstack_networking_secgroup_v2"
	TerraformTypeSecGroupRule    = "openstack_networking_secgroup_rule_v2"
	TerraformTypeBlockVolume     = "openstack_blockstorage_volume_v3"
)

// ParseTerraformState parses a Terraform state JSON file
func ParseTerraformState(r io.Reader) (*TerraformState, error) {
	var state TerraformState
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&state); err != nil {
		return nil, fmt.Errorf("failed to parse Terraform state: %w", err)
	}
	return &state, nil
}

// ParseTerraformStateFile parses a Terraform state JSON file from path
func ParseTerraformStateFile(path string) (*TerraformState, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open Terraform state file: %w", err)
	}
	defer f.Close()
	return ParseTerraformState(f)
}

// ExtractResources extracts unified Resources from Terraform state
func ExtractResourcesFromTerraform(state *TerraformState, projectName string) []Resource {
	if state == nil || state.Values == nil || state.Values.RootModule == nil {
		return nil
	}

	var resources []Resource

	// Case 1: Process direct resources in root module (resources NOT in a module)
	resources = append(resources, extractResourcesFromModule(state.Values.RootModule.Resources, projectName)...)

	// Case 2: Recursively process resources in child modules
	resources = append(resources, extractResourcesFromChildModules(state.Values.RootModule.ChildModules, projectName)...)

	return resources
}

// extractResourcesFromChildModules recursively extracts resources from child modules
func extractResourcesFromChildModules(modules []TerraformChildModule, projectName string) []Resource {
	var resources []Resource
	for _, module := range modules {
		resources = append(resources, extractResourcesFromModule(module.Resources, projectName)...)
		// Recursively process nested child modules
		resources = append(resources, extractResourcesFromChildModules(module.ChildModules, projectName)...)
	}
	return resources
}

// extractResourcesFromModule extracts resources from a slice of TerraformResource
func extractResourcesFromModule(tfResources []TerraformResource, projectName string) []Resource {
	var resources []Resource
	for _, tfRes := range tfResources {
		switch tfRes.Type {
		case TerraformTypeComputeInstance:
			if res := extractServer(tfRes, projectName); res != nil {
				resources = append(resources, *res)
			}
		case TerraformTypeSecurityGroup:
			if res := extractSecurityGroup(tfRes, projectName); res != nil {
				resources = append(resources, *res)
			}
		case TerraformTypeSecGroupRule:
			if res := extractSecurityGroupRule(tfRes, projectName); res != nil {
				resources = append(resources, *res)
			}
		}
	}
	return resources
}

// extractServer extracts a server resource from Terraform resource
func extractServer(tfRes TerraformResource, projectName string) *Resource {
	id := getStringValue(tfRes.Values, "id")
	name := getStringValue(tfRes.Values, "name")

	if id == "" {
		return nil
	}

	// Extract security groups
	var securityGroups []string
	if sgList, ok := tfRes.Values["security_groups"].([]any); ok {
		for _, sg := range sgList {
			if sgName, ok := sg.(string); ok {
				securityGroups = append(securityGroups, sgName)
			}
		}
	}

	// Extract additional properties
	props := make(map[string]any)
	props["ip_address"] = getStringValue(tfRes.Values, "access_ip_v4")
	props["flavor_name"] = getStringValue(tfRes.Values, "flavor_name")
	props["flavor_id"] = getStringValue(tfRes.Values, "flavor_id")
	props["image_name"] = getStringValue(tfRes.Values, "image_name")
	props["power_state"] = getStringValue(tfRes.Values, "power_state")
	props["availability_zone"] = getStringValue(tfRes.Values, "availability_zone")

	return &Resource{
		ID:             id,
		Name:           name,
		Type:           ResourceTypeServer,
		ProjectName:    projectName,
		SecurityGroups: securityGroups,
		Properties:     props,
	}
}

// extractSecurityGroup extracts a security group resource from Terraform resource
func extractSecurityGroup(tfRes TerraformResource, projectName string) *Resource {
	id := getStringValue(tfRes.Values, "id")
	name := getStringValue(tfRes.Values, "name")

	if id == "" {
		return nil
	}

	props := make(map[string]any)
	props["description"] = getStringValue(tfRes.Values, "description")

	return &Resource{
		ID:          id,
		Name:        name,
		Type:        ResourceTypeSecurityGroup,
		ProjectName: projectName,
		Properties:  props,
	}
}

// extractSecurityGroupRule extracts a security group rule resource from Terraform resource
func extractSecurityGroupRule(tfRes TerraformResource, projectName string) *Resource {
	id := getStringValue(tfRes.Values, "id")
	secGroupID := getStringValue(tfRes.Values, "security_group_id")

	if id == "" {
		return nil
	}

	props := make(map[string]any)
	props["direction"] = getStringValue(tfRes.Values, "direction")
	props["ethertype"] = getStringValue(tfRes.Values, "ethertype")
	props["protocol"] = getStringValue(tfRes.Values, "protocol")
	props["remote_ip_prefix"] = getStringValue(tfRes.Values, "remote_ip_prefix")
	props["remote_group_id"] = getStringValue(tfRes.Values, "remote_group_id")

	// Handle port range
	portMin := getIntValue(tfRes.Values, "port_range_min")
	portMax := getIntValue(tfRes.Values, "port_range_max")
	if portMin > 0 || portMax > 0 {
		props["port_range"] = fmt.Sprintf("%d:%d", portMin, portMax)
	}

	return &Resource{
		ID:          id,
		Name:        "", // Rules don't have names in Terraform
		Type:        ResourceTypeSecurityGroupRule,
		ProjectName: projectName,
		ParentID:    secGroupID,
		Properties:  props,
	}
}

// LoadTerraformStateFromDir loads and merges all Terraform state files from a directory
func LoadTerraformStateFromDir(dirPath, projectName string) ([]Resource, error) {
	var allResources []Resource

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .json files
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		state, err := ParseTerraformStateFile(filePath)
		if err != nil {
			// Log warning but continue with other files
			fmt.Printf("Warning: failed to parse %s: %v\n", filePath, err)
			continue
		}

		resources := ExtractResourcesFromTerraform(state, projectName)
		allResources = append(allResources, resources...)
	}

	return allResources, nil
}

// getStringValue safely extracts a string value from a map
func getStringValue(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getIntValue safely extracts an int value from a map
func getIntValue(m map[string]any, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		}
	}
	return 0
}
