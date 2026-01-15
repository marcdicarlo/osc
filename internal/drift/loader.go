package drift

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProjectDir represents a project directory structure
type ProjectDir struct {
	Name     string
	BasePath string
	StatePath string
	TruthPath string
}

// DiscoverProjects finds all project directories in the given base path
// Each project directory should contain 'state' and 'truth' subdirectories
func DiscoverProjects(basePath string) ([]ProjectDir, error) {
	// Verify base path exists
	info, err := os.Stat(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to access base path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("base path is not a directory: %s", basePath)
	}

	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read base directory: %w", err)
	}

	var projects []ProjectDir

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectPath := filepath.Join(basePath, entry.Name())
		statePath := filepath.Join(projectPath, "state")
		truthPath := filepath.Join(projectPath, "truth")

		// Check if both state and truth directories exist
		stateExists := dirExists(statePath)
		truthExists := dirExists(truthPath)

		if !stateExists && !truthExists {
			// Skip directories that don't have either subdirectory
			continue
		}

		projects = append(projects, ProjectDir{
			Name:      entry.Name(),
			BasePath:  projectPath,
			StatePath: statePath,
			TruthPath: truthPath,
		})
	}

	if len(projects) == 0 {
		return nil, fmt.Errorf("no project directories found in %s (expected directories with 'state' and/or 'truth' subdirectories)", basePath)
	}

	return projects, nil
}

// LoadProject loads resources from a single project directory
func LoadProject(project ProjectDir) (state, truth []Resource, err error) {
	// Load state resources
	if dirExists(project.StatePath) {
		state, err = LoadTerraformStateFromDir(project.StatePath, project.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load state for project %s: %w", project.Name, err)
		}
	}

	// Load truth resources
	if dirExists(project.TruthPath) {
		truth, err = LoadTruthFromDir(project.TruthPath, project.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load truth for project %s: %w", project.Name, err)
		}
	}

	return state, truth, nil
}

// ProcessProject loads and compares resources for a single project
func ProcessProject(project ProjectDir) (*ProjectDrift, error) {
	state, truth, err := LoadProject(project)
	if err != nil {
		return nil, err
	}

	// Compare resources
	diffs := CompareResources(state, truth)

	return &ProjectDrift{
		ProjectName: project.Name,
		Drifts:      diffs,
		StateCount:  CountResources(state),
		TruthCount:  CountResources(truth),
	}, nil
}

// ProcessAllProjects processes all projects in the base path
func ProcessAllProjects(basePath string) (*DriftReport, error) {
	projects, err := DiscoverProjects(basePath)
	if err != nil {
		return nil, err
	}

	report := NewDriftReport()

	for _, project := range projects {
		projectDrift, err := ProcessProject(project)
		if err != nil {
			// Log warning but continue with other projects
			fmt.Printf("Warning: failed to process project %s: %v\n", project.Name, err)
			continue
		}
		report.AddProject(*projectDrift)
	}

	return report, nil
}

// EnsureProjectDirs creates the state and truth directories for a project if they don't exist
func EnsureProjectDirs(projectPath string) error {
	statePath := filepath.Join(projectPath, "state")
	truthPath := filepath.Join(projectPath, "truth")

	if err := os.MkdirAll(statePath, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	if err := os.MkdirAll(truthPath, 0755); err != nil {
		return fmt.Errorf("failed to create truth directory: %w", err)
	}

	return nil
}

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
