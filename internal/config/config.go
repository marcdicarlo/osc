package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Config holds the configuration for the application
type Config struct {
	ProjectScope  string        `yaml:"project_scope"`
	ProjectFilter string        `yaml:"project_filter"`
	DBFile        string        `yaml:"db_file"`
	DBTimeout     time.Duration `yaml:"db_timeout"`
	Tables struct {
		Projects      string `yaml:"projects_table"`
		Servers       string `yaml:"servers_table"`
		SecGrps       string `yaml:"secgrps_table"`
		SecGrpRules   string `yaml:"secgrp_rules_table"`
		Volumes       string `yaml:"volumes_table"`
		ServerSecGrps string `yaml:"server_secgrps_table"`
		ServerVolumes string `yaml:"server_volumes_table"`
	} `yaml:"tables"`
	OpenStack struct {
		ComputeService  string `yaml:"compute_service"`
		IdentityService string `yaml:"identity_service"`
		AllTenants      bool   `yaml:"all_tenants"`
		MaxWorkers      int    `yaml:"max_workers"`       // Maximum concurrent workers for API calls (default: 10)
		WorkerTimeout   time.Duration `yaml:"worker_timeout"` // Timeout for individual worker API calls (default: 30s)
	} `yaml:"openstack"`
}

// Load loads the configuration from the given file
// It first checks in the current directory, then in /etc/osc/config.yaml
func Load(file string) (*Config, error) {
	// Try current directory first
	data, err := os.ReadFile(file)
	if err != nil {
		// If not found in current directory, try /etc/osc/config.yaml
		data, err = os.ReadFile("/etc/osc/config.yaml")
		if err != nil {
			return nil, err
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Apply defaults for concurrency settings
	cfg.applyDefaults()

	return &cfg, nil
}

// applyDefaults sets default values for optional configuration fields
func (c *Config) applyDefaults() {
	// Default to 10 concurrent workers if not specified
	if c.OpenStack.MaxWorkers <= 0 {
		c.OpenStack.MaxWorkers = 10
	}

	// Default to 30 second timeout per worker if not specified
	if c.OpenStack.WorkerTimeout == 0 {
		c.OpenStack.WorkerTimeout = 30 * time.Second
	}
}
