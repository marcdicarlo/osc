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
	Tables        struct {
		Projects    string `yaml:"projects_table"`
		Servers     string `yaml:"servers_table"`
		SecGrps     string `yaml:"secgrps_table"`
		SecGrpRules string `yaml:"secgrp_rules_table"`
	} `yaml:"tables"`
	OpenStack struct {
		ComputeService  string `yaml:"compute_service"`
		IdentityService string `yaml:"identity_service"`
		AllTenants      bool   `yaml:"all_tenants"`
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
	return &cfg, nil
}
