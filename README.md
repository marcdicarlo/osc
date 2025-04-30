# OpenStack Cache (osc)

A command-line tool that caches OpenStack resource data locally for improved query performance. This tool maintains a local SQLite database of OpenStack resources and provides fast querying capabilities.

## Features

- Cache OpenStack resources locally (servers, security groups, etc.)
- Fast querying of resources without hitting the OpenStack API
- Project-based filtering and scoping
- Detailed security group rule viewing
- Configurable via YAML configuration file

## Installation

1. Ensure you have Go 1.24 or later installed
2. Clone the repository:

   ```bash
   git clone https://github.com/marcdicarlo/osc.git
   cd osc
   ```

3. Build the project:

   ```bash
   go build
   ```

## Configuration

Create a `config.yaml` file in the project directory:

```yaml
db_file: "cachedb.db"
db_timeout: 60000000000    # 60 seconds
tables:
  projects_table: "os_project_names"
  servers_table: "os_servers"
  secgrps_table: "os_security_groups"
  secgrp_rules_table: "os_security_group_rules"
openstack:
  compute_service: "compute"
  identity_service: "identity"
  all_tenants: true
project_scope: ""      # Limit to specific project (or "all" for all projects)
project_filter: ""     # Comma-separated list of project names to exclude
```

## Usage

### Basic Commands

```bash
# List all servers
osc list servers

# List all security groups
osc list secgrps

# List security groups with their rules
osc list secgrps -r
```

### Filtering and Scoping

The tool provides two ways to filter resources by project:

1. Command-line filtering (temporary):

   ```bash
   # Show servers in projects containing "prod"
   osc list servers -p "prod"

   # Show security groups in projects containing "test"
   osc list secgrps -p "test"
   ```

2. Configuration-based filtering (permanent):

   ```yaml
   # In config.yaml:
   project_scope: "prod-app1"    # Show only resources from this project
   project_filter: "test,dev"    # Exclude projects containing these strings
   ```

### Project Scoping Priority

1. If `project_scope` is set to a project name:
   - Only shows resources from that specific project
   - Command-line filters still work within that scope

2. If `project_scope` is empty or "all":
   - Shows resources from all projects
   - Applies any `project_filter` exclusions
   - Command-line filters work on the full dataset

## Database Schema

The tool uses SQLite with the following table structure:

```sql
CREATE TABLE os_project_names (
    project_id TEXT PRIMARY KEY,
    project_name TEXT NOT NULL
);

CREATE TABLE os_servers (
    server_id TEXT PRIMARY KEY,
    server_name TEXT NOT NULL,
    project_id TEXT,
    FOREIGN KEY (project_id) REFERENCES os_project_names(project_id)
);

CREATE TABLE os_security_groups (
    security_group_id TEXT PRIMARY KEY,
    security_group_name TEXT NOT NULL,
    project_id TEXT,
    FOREIGN KEY (project_id) REFERENCES os_project_names(project_id)
);

CREATE TABLE os_security_group_rules (
    rule_id TEXT PRIMARY KEY,
    security_group_id TEXT,
    direction TEXT NOT NULL,
    protocol TEXT NOT NULL,
    port_range TEXT NOT NULL,
    cidr TEXT NOT NULL,
    FOREIGN KEY (security_group_id) REFERENCES os_security_groups(security_group_id)
);
```

## load dummy data

```sh
sqlite3 cachedb.db < setup.sql
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request
