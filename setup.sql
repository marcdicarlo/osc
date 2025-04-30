-- Enable foreign key constraints (SQLite requires this)
PRAGMA foreign_keys = ON;

-- Drop tables if they exist
DROP TABLE IF EXISTS secgrp_rules;
DROP TABLE IF EXISTS secgrps;
DROP TABLE IF EXISTS servers;
DROP TABLE IF EXISTS projects;

-- Create table for projects
CREATE TABLE projects (
    project_id   TEXT PRIMARY KEY,
    project_name TEXT NOT NULL
);

-- Create table for servers with a foreign key linking to projects
CREATE TABLE servers (
    server_id   TEXT PRIMARY KEY,
    server_name TEXT NOT NULL,
    project_id  TEXT NOT NULL,
    ipv4_addr   TEXT,
    FOREIGN KEY (project_id) REFERENCES projects(project_id) ON DELETE CASCADE
);

-- Create table for security groups with a foreign key linking to projects
CREATE TABLE secgrps (
    secgrp_id   TEXT PRIMARY KEY,
    secgrp_name TEXT NOT NULL,
    project_id  TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(project_id) ON DELETE CASCADE
);

-- Create table for security group rules with a foreign key linking to security groups
CREATE TABLE secgrp_rules (
    rule_id          TEXT PRIMARY KEY,
    secgrp_id       TEXT NOT NULL,
    direction       TEXT NOT NULL,
    ethertype       TEXT NOT NULL,
    protocol        TEXT,
    port_range_min  INTEGER,
    port_range_max  INTEGER,
    remote_ip_prefix TEXT,
    FOREIGN KEY (secgrp_id) REFERENCES secgrps(secgrp_id) ON DELETE CASCADE
);

-- Insert dummy data into projects
INSERT INTO projects (project_id, project_name) VALUES 
    ('proj-1', 'hc_alpha_project'),
    ('proj-2', 'hc_beta_project'),
    ('proj-3', 'hc_gamma_project'),
    ('proj-4', 'hc_delta_project'),
    ('proj-5', 'hc_epsilon_project'),
    ('proj-6', 'hc_zeta_project'),
    ('proj-7', 'hc_eta_project');

-- Insert dummy data into servers with IPv4 addresses
INSERT INTO servers (server_id, server_name, project_id, ipv4_addr) VALUES 
    ('srv-101', 'sa1x-server-p1', 'proj-1', '192.168.1.101'),
    ('srv-102', 'sa1x-server-p2', 'proj-1', '192.168.1.102'),
    ('srv-103', 'sa1x-server-p3', 'proj-2', '192.168.2.103'),
    ('srv-104', 'sa1x-server-p4', 'proj-2', '192.168.2.104'),
    ('srv-105', 'sa1x-server-p5', 'proj-3', '192.168.3.105'),
    ('srv-106', 'sa1x-server-p6', 'proj-3', '192.168.3.106'),
    ('srv-107', 'sa1x-server-p7', 'proj-4', '192.168.4.107'),
    ('srv-108', 'sa1x-server-p8', 'proj-4', '192.168.4.108'),
    ('srv-109', 'sa1x-server-p9', 'proj-5', '192.168.5.109'),
    ('srv-110', 'sa1x-server-p10', 'proj-5', '192.168.5.110'),
    ('srv-111', 'sa1x-server-p11', 'proj-6', '192.168.6.111'),
    ('srv-112', 'sa1x-server-p12', 'proj-6', '192.168.6.112'),
    ('srv-113', 'sa1x-server-p13', 'proj-7', '192.168.7.113'),
    ('srv-114', 'sa1x-server-p14', 'proj-7', '192.168.7.114');

-- Insert dummy data into security groups
INSERT INTO secgrps (secgrp_id, secgrp_name, project_id) VALUES 
    ('sg-1', 'default', 'proj-1'),
    ('sg-2', 'web-servers', 'proj-1'),
    ('sg-3', 'default', 'proj-2'),
    ('sg-4', 'database', 'proj-2'),
    ('sg-5', 'default', 'proj-3');

-- Insert dummy data into security group rules
INSERT INTO secgrp_rules (rule_id, secgrp_id, direction, ethertype, protocol, port_range_min, port_range_max, remote_ip_prefix) VALUES 
    ('rule-1', 'sg-1', 'ingress', 'IPv4', 'tcp', 22, 22, '0.0.0.0/0'),
    ('rule-2', 'sg-1', 'egress', 'IPv4', 'tcp', NULL, NULL, '0.0.0.0/0'),
    ('rule-3', 'sg-2', 'ingress', 'IPv4', 'tcp', 80, 80, '0.0.0.0/0'),
    ('rule-4', 'sg-2', 'ingress', 'IPv4', 'tcp', 443, 443, '0.0.0.0/0'),
    ('rule-5', 'sg-3', 'ingress', 'IPv4', 'tcp', 22, 22, '10.0.0.0/8'),
    ('rule-6', 'sg-4', 'ingress', 'IPv4', 'tcp', 5432, 5432, '172.16.0.0/12'),
    ('rule-7', 'sg-4', 'ingress', 'IPv4', 'tcp', 3306, 3306, '172.16.0.0/12');


