package drift

// ResourceType represents the type of OpenStack resource
type ResourceType string

const (
	ResourceTypeServer            ResourceType = "server"
	ResourceTypeSecurityGroup     ResourceType = "security-group"
	ResourceTypeSecurityGroupRule ResourceType = "security-group-rule"
)

// DriftStatus represents the type of drift detected
type DriftStatus string

const (
	StatusMissingInTruth  DriftStatus = "missing_in_truth"
	StatusMissingInState  DriftStatus = "missing_in_state"
	StatusNameChanged     DriftStatus = "name_changed"
	StatusSecGroupChanged DriftStatus = "secgroups_changed"
	StatusRuleChanged     DriftStatus = "rule_changed"
)

// Resource represents a unified resource from either Terraform state or osc truth
type Resource struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Type           ResourceType           `json:"type"`
	ProjectName    string                 `json:"project_name"`
	ParentID       string                 `json:"parent_id,omitempty"`       // For rules: parent security group ID
	ParentName     string                 `json:"parent_name,omitempty"`     // For rules: parent security group name
	SecurityGroups []string               `json:"security_groups,omitempty"` // For servers: attached security group names
	Properties     map[string]any `json:"properties,omitempty"` // Additional properties for detailed comparison
}

// DiffResult represents a single drift detection result
type DiffResult struct {
	ResourceType ResourceType `json:"resource_type"`
	ResourceName string       `json:"resource_name"`
	ResourceID   string       `json:"resource_id"`
	ProjectName  string       `json:"project_name"`
	ParentSG     string       `json:"parent_sg,omitempty"` // For rules only
	Status       DriftStatus  `json:"status"`
	Details      string       `json:"details"` // Description of what changed
}

// ProjectDrift holds drift detection results for a single project
type ProjectDrift struct {
	ProjectName string       `json:"project_name"`
	Drifts      []DiffResult `json:"drifts"`
	StateCount  ResourceCounts `json:"state_count"`
	TruthCount  ResourceCounts `json:"truth_count"`
}

// ResourceCounts holds counts of resources by type
type ResourceCounts struct {
	Servers            int `json:"servers"`
	SecurityGroups     int `json:"security_groups"`
	SecurityGroupRules int `json:"security_group_rules"`
}

// DriftReport holds the complete drift detection report
type DriftReport struct {
	Projects []ProjectDrift `json:"projects"`
	Summary  DriftSummary   `json:"summary"`
}

// DriftSummary provides aggregate statistics
type DriftSummary struct {
	TotalProjects int                    `json:"total_projects"`
	TotalDrift    int                    `json:"total_drift"`
	ByStatus      map[DriftStatus]int    `json:"by_status"`
	ByType        map[ResourceType]int   `json:"by_type"`
}

// NewDriftReport creates a new empty DriftReport
func NewDriftReport() *DriftReport {
	return &DriftReport{
		Projects: make([]ProjectDrift, 0),
		Summary: DriftSummary{
			ByStatus: make(map[DriftStatus]int),
			ByType:   make(map[ResourceType]int),
		},
	}
}

// AddProject adds a project's drift results to the report
func (r *DriftReport) AddProject(project ProjectDrift) {
	r.Projects = append(r.Projects, project)
	r.Summary.TotalProjects++
	r.Summary.TotalDrift += len(project.Drifts)

	for _, drift := range project.Drifts {
		r.Summary.ByStatus[drift.Status]++
		r.Summary.ByType[drift.ResourceType]++
	}
}

// HasDrift returns true if any drift was detected
func (r *DriftReport) HasDrift() bool {
	return r.Summary.TotalDrift > 0
}
