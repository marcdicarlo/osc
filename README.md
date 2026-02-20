# OpenStack Cache (osc)

A command-line tool that caches OpenStack resource data locally for improved query performance. This tool maintains a local SQLite database of OpenStack resources and provides fast querying capabilities.

## Features

- Cache OpenStack resources locally (servers, security groups, etc.)
- Fast querying of resources without hitting the OpenStack API
- Project-based filtering and scoping
- Detailed resource views:
  - `show server` - Server details including status, image, flavor, volumes, metadata
  - `show secgrp` - Security group details with rules and attached servers
- Server listing with security groups:
  - `--rules` flag shows security group names
  - `--full` flag shows security group IDs and names
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
   make build
   ```

   `make build` and `make test` use repository-local Go caches (`.cache/go-build` and `.cache/go-mod`) by default. This avoids permission issues in restricted or sandboxed environments while still allowing overrides via environment or make variable assignments.

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

### Syncing Resources

Before querying resources, you need to sync data from OpenStack to the local cache:

```bash
# Sync all resources from all projects
osc sync all

# Sync resources for a specific project (supports partial matching)
osc sync project production-web
osc sync project prod

# The project name is matched case-insensitively
# If multiple projects match, you'll be asked to be more specific
# If one project matches, you'll see a confirmation and the sync will proceed
```

### Basic Commands

```bash
# List all servers
osc list servers

# List servers with security group names
osc list servers --rules

# List servers with security group IDs and names
osc list servers --full

# List all security groups
osc list secgrps

# List security groups with their rules
osc list secgrps -r

# Show detailed information for a specific server
osc show server my-server-name

# Show detailed information for a specific security group
osc show secgrp web-servers

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

   ```bash
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

### Servers

The servers command (`osc list servers`) supports displaying security groups attached to each server:

1. Basic listing:

   ```bash
   # List servers without security groups
   osc list servers
   ```

2. Include security group names:

   ```bash
   # List servers with security group names only
   osc list servers --rules
   osc list servers -r
   ```

   Example output:

   ```bash
   SERVER NAME     | SERVER ID | PROJECT NAME     | IPV4 ADDRESS  | SECURITY GROUPS
   sa1x-server-p1  | srv-101   | hc_alpha_project | 192.168.1.101 | default, web-servers
   ```

3. Include security group IDs and names:

   ```bash
   # Full output with IDs (useful for scripting)
   osc list servers --full
   osc list servers -f
   ```

   Example output:

   ```bash
   SERVER NAME     | SERVER ID | PROJECT NAME     | IPV4 ADDRESS  | SECURITY GROUPS
   sa1x-server-p1  | srv-101   | hc_alpha_project | 192.168.1.101 | sg-1 (default), sg-2 (web-servers)
   ```

4. JSON output with security groups:

   ```bash
   osc list servers --rules -o json
   ```

   Security groups are output as a list:

   ```json
   {
     "fields": {
       "Server Name": "sa1x-server-p1",
       "Server ID": "srv-101",
       "Project Name": "hc_alpha_project",
       "IPv4 Address": "192.168.1.101"
     },
     "security_groups": ["default", "web-servers"]
   }
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

3. Show full rule details (includes ethertype and remote security groups):

   ```bash
   # Full details including ethertype and remote group IDs
   osc list secgrps -r --full
   osc list secgrps -r -f

   # Full output in different formats
   osc list secgrps -r --full -o json
   osc list secgrps -r -f -o csv
   ```

4. Rule details:
   - **Basic mode** (`-r`):
     - Direction (ingress/egress)
     - Protocol (tcp, udp, icmp, or 'any')
     - Port Range (single port, range, or 'any')
     - Remote IP (CIDR or 'any')

   - **Full mode** (`-r --full` or `-r -f`):
     - All basic fields plus:
     - Ethertype (IPv4/IPv6)
     - Remote Group (source security group ID with resolved name)

   Example full output:

   ```bash
   rule-9  | sg-4 | ... | ingress | tcp | 3306 | any | IPv4 | sg-2 (web-servers)
   ```

5. Resource types:
   - `security-group`: The security group itself
   - `security-group-rule`: Individual rules within a group

### Show Commands

The `show` commands provide detailed information about specific resources:

#### Show Server

Display detailed information about a specific server:

```bash
# Show server details
osc show server my-server-name

# Show server in a specific project
osc show server my-server-name -p prod

# Output in different formats
osc show server my-server-name -o json
osc show server my-server-name -o csv
```

Server details include:

- Server ID, name, and project
- Status (ACTIVE, SHUTOFF, etc.)
- IPv4 address
- Image ID and name
- Flavor ID and name
- Attached volumes
- Metadata (key-value pairs)
- Security groups

Example table output:

```bash
Server: my-server-name
  ID:      srv-101
  Project: hc_alpha_project (proj-alpha)
  Status:  ACTIVE
  IPv4:    192.168.1.101
  Image:   img-ubuntu-22 (Ubuntu 22.04 LTS)
  Flavor:  flv-medium (m1.medium)

  Volumes:
    - vol-001 (boot-volume)

  Metadata:
    - environment: production
    - owner: devops

  Security Groups:
    - sg-1 (default)
    - sg-2 (web-servers)
```

#### Show Security Group

Display detailed information about a specific security group:

```bash
# Show security group details
osc show secgrp web-servers

# Show security group in a specific project
osc show secgrp web-servers -p prod

# Output in different formats
osc show secgrp web-servers -o json
osc show secgrp web-servers -o csv
```

Security group details include:

- Security group ID, name, and project
- All rules (direction, protocol, ports, remote IP/group)
- Servers using this security group

Example table output:

```bash
Security Group: web-servers
  ID:      sg-2
  Project: hc_alpha_project (proj-alpha)

  Rules:
    INGRESS tcp  80    from 0.0.0.0/0
    INGRESS tcp  443   from 0.0.0.0/0
    EGRESS  any  any   to   any

  Servers Using This Group:
    - sa1x-server-p1 (srv-101)
    - sa1x-server-p2 (srv-102)
```

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

Contributions are welcome! Following this outline helps keep work fast and reviews predictable:

1. **Start with an issue.** Open a GitHub issue describing bugs, feature ideas, or docs improvements so we can agree on scope before you code. Link relevant logs/output and tag `bug` or `feature` appropriately.
2. **Branch from `main`.** Create a branch named `username/XX-brief-description` (replace `XX` with the issue or ticket number). Rebase frequently so your work keeps up with `main`.
3. **Build the change.** Write tests (unit, integration, regression) and ensure `go test ./...` passes locally. Run `go fmt` on files you touched and lint any Go files if applicable.
4. **Document the work.** Update `README.md`, config snippets, or other docs to describe new commands, configuration options, or behaviours. Mention when migrations/backwards compatibility changes are required.
5. **Prepare the PR.** Push the branch, reference the issue in the description, explain testing steps, and summarize the change. Use small, focused commits with clear messages.

Before opening a pull request, double-check that:

- The new behaviour is covered by automated tests or a justification is provided.
- Any configuration or schema changes have been documented in this README or a dedicated migration guide.
- The code follows the existing style (align tabs/spaces, idiomatic Go, no commented-out code).
- Sensitive or credential data is not stored in the repo (use env vars or config files as needed).

After your PR is open, respond to review feedback, squash or fixup commits if requested, and confirm CI/test jobs pass if applicable.

## License

OpenStack Cache is licensed under the Apache License, Version 2.0. See `LICENSE` for the full text.
