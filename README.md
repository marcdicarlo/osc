# OpenStack Cache (osc)

A command-line tool that caches OpenStack resource data locally for improved query performance. This tool maintains a local SQLite database of OpenStack resources and provides fast querying capabilities.

## Features

- Cache OpenStack resources locally (servers, security groups, etc.)
- Fast querying of resources without hitting the OpenStack API
- Project-based filtering and scoping
- Detailed security group rule viewing:
  - Unified view of groups and rules
  - Rule details including direction, protocol, ports, and CIDR
  - Filtering applies to both groups and rules
- Multiple output formats:
  - Human-readable tables (default)
  - Structured JSON with metadata and type information
  - RFC 4180 compliant CSV with headers
- Rich metadata in output:
  - Project filtering results
  - Resource type information
  - Relationship between groups and rules
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

# Filter resources by project name
osc list servers -p "prod"
osc list secgrps -p "test"

# Combine filtering and output formats
osc list secgrps -p "prod" -r -o json
```

### Output Formats

The tool supports three output formats, controlled by the `-o` or `--output` flag:

1. Table format (default):
   ```bash
   # Default table output
   osc list servers
   
   # Explicit table output
   osc list servers -o table
   ```

   Example table output:
   ```
   Name        ID        Project ID   Project Name   Resource Type
   default     sg-123    proj-123    prod-app1      security-group
   ssh-rule    sg-123    proj-123    prod-app1      security-group-rule
   ```

2. JSON format:
   ```bash
   # Output in JSON format
   osc list servers -o json
   
   # Security groups with rules in JSON
   osc list secgrps -r -o json
   ```
   
   JSON output includes metadata and structured fields:
   ```json
   {
     "metadata": {
       "filtering": {
         "filtered_project_count": 2,
         "matched_projects": ["prod-app1", "prod-app2"]
       }
     },
     "headers": ["Name", "ID", "Project ID", "Project Name", "Resource Type"],
     "data": [
       {
         "type": "security-group",
         "fields": {
           "Name": "default",
           "ID": "sg-123",
           "Project ID": "proj-123",
           "Project Name": "prod-app1",
           "Resource Type": "security-group"
         }
       },
       {
         "type": "security-group-rule",
         "fields": {
           "Name": "ssh-rule",
           "ID": "sg-123",
           "Project ID": "proj-123",
           "Project Name": "prod-app1",
           "Resource Type": "security-group-rule"
         },
         "rule_fields": {
           "direction": "ingress",
           "protocol": "tcp",
           "port_range": "22",
           "remote_ip": "0.0.0.0/0"
         }
       }
     ]
   }
   ```

3. CSV format:
   ```bash
   # Output in CSV format
   osc list servers -o csv
   
   # Security groups with rules in CSV
   osc list secgrps -r -o csv
   ```
   
   CSV output includes headers and is RFC 4180 compliant:
   ```csv
   Name,ID,Project ID,Project Name,Resource Type,Direction,Protocol,Port Range,Remote IP
   default,sg-123,proj-123,prod-app1,security-group,,,,
   ssh-rule,sg-123,proj-123,prod-app1,security-group-rule,ingress,tcp,22,0.0.0.0/0
   ```

### Security Groups

The security groups command (`osc list secgrps`) provides a unified view of security groups and their rules:

1. Basic listing:
   ```bash
   # List only security groups
   osc list secgrps
   ```

2. Include rules:
   ```bash
   # List groups with their rules
   osc list secgrps -r
   ```

3. Rule details include:
   - Direction (ingress/egress)
   - Protocol (tcp, udp, icmp, or 'any')
   - Port Range (single port, range, or 'any')
   - Remote IP (CIDR or 'any')

4. Resource types:
   - `security-group`: The security group itself
   - `security-group-rule`: Individual rules within a group

### Filtering and Scoping

The tool provides two ways to filter resources by project:

1. Command-line filtering (temporary):
   ```bash
   # Show servers in projects containing "prod"
   osc list servers -p "prod"

   # Show security groups in projects containing "test"
   osc list secgrps -p "test"

   # Show filtered security groups with rules in JSON
   osc list secgrps -p "prod" -r -o json
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

## Development

### Loading Test Data

To load sample data for testing:

```sh
sqlite3 cachedb.db < setup.sql
```

### Running Tests

To run the test suite:

```sh
go test ./...
```

The test suite includes:
- Unit tests for all formatters
- Integration tests for commands
- Project filtering tests
- Output format validation

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

When contributing, please ensure:
- All tests pass
- New features include tests
- Documentation is updated
- Code follows existing style
