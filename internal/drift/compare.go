package drift

import (
	"fmt"
	"sort"
	"strings"
)

// CompareResources compares state and truth resources and returns drift results
func CompareResources(state, truth []Resource) []DiffResult {
	var results []DiffResult

	// Group resources by type for comparison
	stateByType := groupByType(state)
	truthByType := groupByType(truth)

	// Compare servers
	serverDiffs := compareResourcesByType(
		stateByType[ResourceTypeServer],
		truthByType[ResourceTypeServer],
		compareServerProperties,
	)
	results = append(results, serverDiffs...)

	// Compare security groups
	secGroupDiffs := compareResourcesByType(
		stateByType[ResourceTypeSecurityGroup],
		truthByType[ResourceTypeSecurityGroup],
		compareSecurityGroupProperties,
	)
	results = append(results, secGroupDiffs...)

	// Compare security group rules
	ruleDiffs := compareResourcesByType(
		stateByType[ResourceTypeSecurityGroupRule],
		truthByType[ResourceTypeSecurityGroupRule],
		compareSecurityGroupRuleProperties,
	)
	results = append(results, ruleDiffs...)

	return results
}

// PropertyComparer is a function that compares two resources and returns diff details
type PropertyComparer func(stateRes, truthRes *Resource) (DriftStatus, string)

// compareResourcesByType compares resources of a specific type using ID-based matching
func compareResourcesByType(stateResources, truthResources []Resource, propComparer PropertyComparer) []DiffResult {
	var results []DiffResult

	// Create maps for O(1) lookup
	stateByID := make(map[string]*Resource)
	truthByID := make(map[string]*Resource)

	for i := range stateResources {
		stateByID[stateResources[i].ID] = &stateResources[i]
	}
	for i := range truthResources {
		truthByID[truthResources[i].ID] = &truthResources[i]
	}

	// Find resources in state but not in truth
	for id, stateRes := range stateByID {
		if _, exists := truthByID[id]; !exists {
			results = append(results, DiffResult{
				ResourceType: stateRes.Type,
				ResourceName: stateRes.Name,
				ResourceID:   stateRes.ID,
				ProjectName:  stateRes.ProjectName,
				ParentSG:     getParentSG(stateRes),
				Status:       StatusMissingInTruth,
				Details:      "Resource exists in Terraform state but not in OpenStack",
			})
		}
	}

	// Find resources in truth but not in state
	for id, truthRes := range truthByID {
		if _, exists := stateByID[id]; !exists {
			results = append(results, DiffResult{
				ResourceType: truthRes.Type,
				ResourceName: truthRes.Name,
				ResourceID:   truthRes.ID,
				ProjectName:  truthRes.ProjectName,
				ParentSG:     getParentSG(truthRes),
				Status:       StatusMissingInState,
				Details:      "Resource exists in OpenStack but not in Terraform state",
			})
		}
	}

	// Compare matching resources for property changes
	for id, stateRes := range stateByID {
		if truthRes, exists := truthByID[id]; exists {
			if status, details := propComparer(stateRes, truthRes); status != "" {
				results = append(results, DiffResult{
					ResourceType: stateRes.Type,
					ResourceName: stateRes.Name,
					ResourceID:   stateRes.ID,
					ProjectName:  stateRes.ProjectName,
					ParentSG:     getParentSG(stateRes),
					Status:       status,
					Details:      details,
				})
			}
		}
	}

	return results
}

// compareServerProperties compares server properties between state and truth
func compareServerProperties(stateRes, truthRes *Resource) (DriftStatus, string) {
	var changes []string

	// Check name change
	if stateRes.Name != truthRes.Name {
		changes = append(changes, fmt.Sprintf("name: %q -> %q", stateRes.Name, truthRes.Name))
	}

	// Check security group changes
	stateSGs := normalizeSecurityGroups(stateRes.SecurityGroups)
	truthSGs := normalizeSecurityGroups(truthRes.SecurityGroups)

	if !stringSlicesEqual(stateSGs, truthSGs) {
		added, removed := diffStringSlices(stateSGs, truthSGs)
		if len(added) > 0 || len(removed) > 0 {
			var sgChanges []string
			if len(added) > 0 {
				sgChanges = append(sgChanges, fmt.Sprintf("added: %v", added))
			}
			if len(removed) > 0 {
				sgChanges = append(sgChanges, fmt.Sprintf("removed: %v", removed))
			}
			changes = append(changes, fmt.Sprintf("security_groups: %s", strings.Join(sgChanges, ", ")))
		}
	}

	if len(changes) == 0 {
		return "", ""
	}

	// Determine the most specific status
	if stateRes.Name != truthRes.Name {
		return StatusNameChanged, strings.Join(changes, "; ")
	}
	return StatusSecGroupChanged, strings.Join(changes, "; ")
}

// compareSecurityGroupProperties compares security group properties
func compareSecurityGroupProperties(stateRes, truthRes *Resource) (DriftStatus, string) {
	if stateRes.Name != truthRes.Name {
		return StatusNameChanged, fmt.Sprintf("name: %q -> %q", stateRes.Name, truthRes.Name)
	}
	return "", ""
}

// compareSecurityGroupRuleProperties compares security group rule properties
// Note: We only match rules by ID; we don't compare detailed properties like
// direction, protocol, port_range, etc. since the truth file doesn't include them.
func compareSecurityGroupRuleProperties(stateRes, truthRes *Resource) (DriftStatus, string) {
	// Only match by ID - don't compare rule properties
	return "", ""
}

// Helper functions

// getParentSG returns the parent security group identifier (name or ID)
func getParentSG(res *Resource) string {
	if res.ParentName != "" {
		return res.ParentName
	}
	return res.ParentID
}

// groupByType groups resources by their type
func groupByType(resources []Resource) map[ResourceType][]Resource {
	result := make(map[ResourceType][]Resource)
	for _, res := range resources {
		result[res.Type] = append(result[res.Type], res)
	}
	return result
}

// normalizeSecurityGroups sorts and deduplicates security group names
func normalizeSecurityGroups(sgs []string) []string {
	if len(sgs) == 0 {
		return nil
	}

	// Deduplicate
	seen := make(map[string]bool)
	var result []string
	for _, sg := range sgs {
		if !seen[sg] {
			seen[sg] = true
			result = append(result, sg)
		}
	}

	// Sort for consistent comparison
	sort.Strings(result)
	return result
}

// stringSlicesEqual checks if two string slices are equal
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// diffStringSlices returns slices of added and removed elements
func diffStringSlices(state, truth []string) (added, removed []string) {
	stateSet := make(map[string]bool)
	truthSet := make(map[string]bool)

	for _, s := range state {
		stateSet[s] = true
	}
	for _, s := range truth {
		truthSet[s] = true
	}

	// Find elements in truth but not in state (added)
	for _, s := range truth {
		if !stateSet[s] {
			added = append(added, s)
		}
	}

	// Find elements in state but not in truth (removed)
	for _, s := range state {
		if !truthSet[s] {
			removed = append(removed, s)
		}
	}

	return added, removed
}

// getPropertyString safely gets a string property value
func getPropertyString(props map[string]any, key string) string {
	if props == nil {
		return ""
	}
	if v, ok := props[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// normalizeRuleValue normalizes rule property values for comparison
func normalizeRuleValue(val string) string {
	val = strings.TrimSpace(val)
	// Normalize "any" and empty values
	if val == "" || val == "any" || val == "n/a" {
		return ""
	}
	return val
}

// CountResources counts resources by type
func CountResources(resources []Resource) ResourceCounts {
	counts := ResourceCounts{}
	for _, res := range resources {
		switch res.Type {
		case ResourceTypeServer:
			counts.Servers++
		case ResourceTypeSecurityGroup:
			counts.SecurityGroups++
		case ResourceTypeSecurityGroupRule:
			counts.SecurityGroupRules++
		}
	}
	return counts
}
