package filter

import (
	"fmt"
	"strings"

	"github.com/marcdicarlo/osc/internal/config"
)

// ProjectFilter holds the configuration for filtering projects
type ProjectFilter struct {
	// Filter string from command line flag (projects to exclude)
	FlagFilter string
	// Configuration from config file
	Config *config.Config
}

// New creates a new ProjectFilter
func New(flagFilter string, cfg *config.Config) *ProjectFilter {
	return &ProjectFilter{
		FlagFilter: flagFilter,
		Config:     cfg,
	}
}

// GetActiveFilter returns the active filter string to use
// Command line flag takes precedence over config file
func (pf *ProjectFilter) GetActiveFilter() string {
	if pf.FlagFilter != "" {
		return pf.FlagFilter
	}
	return pf.Config.ProjectFilter
}

// shouldIncludeProject determines if a project should be included based on scope and filter
func (pf *ProjectFilter) shouldIncludeProject(projectName string) bool {
	// First check project scope
	if pf.Config.ProjectScope != "" && pf.Config.ProjectScope != "all" {
		return strings.EqualFold(projectName, pf.Config.ProjectScope)
	}

	// If we get here, project is in scope. Now check filters

	// First apply config file exclusion filter
	if pf.Config.ProjectFilter != "" {
		excludeProjects := strings.Split(pf.Config.ProjectFilter, ",")
		for _, exclude := range excludeProjects {
			exclude = strings.TrimSpace(exclude)
			if strings.Contains(strings.ToLower(projectName), strings.ToLower(exclude)) {
				return false
			}
		}
	}

	// Then apply command line inclusion filter if present
	if pf.FlagFilter != "" {
		return strings.Contains(strings.ToLower(projectName), strings.ToLower(pf.FlagFilter))
	}

	return true
}

// MatchProjects filters a slice of project data based on scope and filter settings
// Returns the filtered data and a map of matched project names
func (pf *ProjectFilter) MatchProjects(data [][]string, projectNameIndex int) ([][]string, map[string]bool) {
	matchedProjects := make(map[string]bool)
	var filteredData [][]string

	for _, row := range data {
		if projectNameIndex >= len(row) {
			continue
		}
		pname := row[projectNameIndex]
		if pf.shouldIncludeProject(pname) {
			matchedProjects[pname] = true
			filteredData = append(filteredData, row)
		}
	}

	return filteredData, matchedProjects
}

// FormatMatchedProjects returns a formatted string describing which projects were matched
func (pf *ProjectFilter) FormatMatchedProjects(matchedProjects map[string]bool, resourceType string) string {
	if len(matchedProjects) == 0 {
		if pf.Config.ProjectScope != "" && pf.Config.ProjectScope != "all" {
			return fmt.Sprintf("No %s found in project: %s\n", resourceType, pf.Config.ProjectScope)
		}
		if pf.FlagFilter != "" {
			return fmt.Sprintf("No %s found in projects containing: %s\n", resourceType, pf.FlagFilter)
		}
		return fmt.Sprintf("No %s found\n", resourceType)
	}

	matchedList := make([]string, 0, len(matchedProjects))
	for pname := range matchedProjects {
		matchedList = append(matchedList, pname)
	}

	var msg string
	if pf.Config.ProjectScope != "" && pf.Config.ProjectScope != "all" {
		msg = fmt.Sprintf("Showing %s in project '%s'", resourceType, pf.Config.ProjectScope)
	} else if pf.FlagFilter != "" {
		msg = fmt.Sprintf("Showing %s in projects containing '%s'", resourceType, pf.FlagFilter)
	} else if pf.Config.ProjectFilter != "" {
		excludeList := strings.Split(pf.Config.ProjectFilter, ",")
		for i, name := range excludeList {
			excludeList[i] = fmt.Sprintf("'%s'", strings.TrimSpace(name))
		}
		msg = fmt.Sprintf("Showing %s (excluding projects containing: %s)", resourceType, strings.Join(excludeList, ", "))
	} else {
		msg = fmt.Sprintf("Showing %s in all projects", resourceType)
	}

	return fmt.Sprintf("%s:\n  Projects: %s\n", msg, strings.Join(matchedList, ", "))
}
