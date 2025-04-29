-- Enable foreign key constraints (SQLite requires this)
PRAGMA foreign_keys = ON;

-- Drop tables if they already exist, for a clean start.
DROP TABLE IF EXISTS os_servers;
DROP TABLE IF EXISTS os_project_names;
DROP TABLE IF EXISTS os_security_groups;
DROP TABLE IF EXISTS os_security_group_rules;

-- Create table for project names.
CREATE TABLE os_project_names (
    project_id TEXT PRIMARY KEY,
    project_name TEXT NOT NULL
);

-- Create table for servers with a foreign key linking to os_project_names.
CREATE TABLE os_servers (
    server_id TEXT PRIMARY KEY,
    server_name TEXT NOT NULL,
    project_id TEXT,
    FOREIGN KEY (project_id) REFERENCES os_project_names(project_id)
);

-- Create table for security groups with a foreign key linking to os_project_names.
CREATE TABLE os_security_groups (
    security_group_id TEXT PRIMARY KEY,
    security_group_name TEXT NOT NULL,
    project_id TEXT,
    FOREIGN KEY (project_id) REFERENCES os_project_names(project_id)
);

-- Create table for security group rules with a foreign key linking to os_security_groups.
CREATE TABLE os_security_group_rules (
    rule_id TEXT PRIMARY KEY,
    security_group_id TEXT,
    direction TEXT NOT NULL,
    protocol TEXT NOT NULL,
    port_range TEXT NOT NULL,
    cidr TEXT NOT NULL,
    FOREIGN KEY (security_group_id) REFERENCES os_security_groups(security_group_id)
);

-- Insert dummy data into os_project_names.
INSERT INTO os_project_names (project_id, project_name) VALUES (1, 'hc_alpha_project');
INSERT INTO os_project_names (project_id, project_name) VALUES (2, 'hc_beta_project');
INSERT INTO os_project_names (project_id, project_name) VALUES (3, 'hc_gamma_project');
INSERT INTO os_project_names (project_id, project_name) VALUES (4, 'hc_delta_project');
INSERT INTO os_project_names (project_id, project_name) VALUES (5, 'hc_epsilon_project');

-- Insert dummy data into os_servers, linking each server to a project.
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (101, 'sa1x-server-p1', 1);
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (102, 'sa1x-server-p2', 1);
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (103, 'sa1x-server-p3', 2);
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (104, 'sa1x-server-p4', 2);
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (105, 'sa1x-server-p5', 3);
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (106, 'sa1x-server-p6', 3);
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (107, 'sa1x-server-p7', 4);
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (108, 'sa1x-server-p8', 4);
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (109, 'sa1x-server-p9', 5);
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (110, 'sa1x-server-p10', 5);

-- Optional: Insert additional dummy data for more variety.
INSERT INTO os_project_names (project_id, project_name) VALUES (6, 'hc_zeta_project');
INSERT INTO os_project_names (project_id, project_name) VALUES (7, 'hc_eta_project');

INSERT INTO os_servers (server_id, server_name, project_id) VALUES (111, 'sa1x-server-p11', 6);
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (112, 'sa1x-server-p12', 6);
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (113, 'sa1x-server-p13', 7);
INSERT INTO os_servers (server_id, server_name, project_id) VALUES (114, 'sa1x-server-p14', 7);

-- insert dummy data into os_security_groups
INSERT INTO os_security_groups (security_group_id, security_group_name, project_id) VALUES (1, 'hc_alpha_security_group', 1);
INSERT INTO os_security_groups (security_group_id, security_group_name, project_id) VALUES (2, 'hc_beta_security_group', 2);
INSERT INTO os_security_groups (security_group_id, security_group_name, project_id) VALUES (3, 'hc_gamma_security_group', 3);
INSERT INTO os_security_groups (security_group_id, security_group_name, project_id) VALUES (4, 'hc_delta_security_group', 4);
INSERT INTO os_security_groups (security_group_id, security_group_name, project_id) VALUES (5, 'hc_epsilon_security_group', 5);

-- insert dummy data into os_security_group_rules
INSERT INTO os_security_group_rules (rule_id, security_group_id, direction, protocol, port_range, cidr) VALUES (1, 1, 'inbound', 'tcp', '80', '0.0.0.0/0');
INSERT INTO os_security_group_rules (rule_id, security_group_id, direction, protocol, port_range, cidr) VALUES (2, 1, 'outbound', 'tcp', '80', '0.0.0.0/0');
INSERT INTO os_security_group_rules (rule_id, security_group_id, direction, protocol, port_range, cidr) VALUES (3, 2, 'inbound', 'tcp', '80', '0.0.0.0/0');
INSERT INTO os_security_group_rules (rule_id, security_group_id, direction, protocol, port_range, cidr) VALUES (4, 2, 'outbound', 'tcp', '80', '0.0.0.0/0');


