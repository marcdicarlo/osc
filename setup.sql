-- Enable foreign key constraints (SQLite requires this)
PRAGMA foreign_keys = ON;

-- Drop tables if they exist (junction tables first due to FK constraints)
DROP TABLE IF EXISTS os_server_volumes;
DROP TABLE IF EXISTS os_server_secgrps;
DROP TABLE IF EXISTS os_volumes;
DROP TABLE IF EXISTS os_security_group_rules;
DROP TABLE IF EXISTS os_security_groups;
DROP TABLE IF EXISTS os_servers;
DROP TABLE IF EXISTS os_project_names;

-- Create table for projects
CREATE TABLE os_project_names (
    project_id   TEXT PRIMARY KEY,
    project_name TEXT NOT NULL
);

-- Create table for servers with a foreign key linking to projects
CREATE TABLE os_servers (
    server_id   TEXT PRIMARY KEY,
    server_name TEXT NOT NULL,
    project_id  TEXT NOT NULL,
    ipv4_addr   TEXT,
    status      TEXT,
    image_id    TEXT,
    image_name  TEXT,
    flavor_id   TEXT,
    flavor_name TEXT,
    metadata    TEXT,
    FOREIGN KEY (project_id) REFERENCES os_project_names(project_id) ON DELETE CASCADE
);

-- Create table for security groups with a foreign key linking to projects
CREATE TABLE os_security_groups (
    secgrp_id   TEXT PRIMARY KEY,
    secgrp_name TEXT NOT NULL,
    project_id  TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES os_project_names(project_id) ON DELETE CASCADE
);

-- Create table for security group rules with a foreign key linking to security groups
CREATE TABLE os_security_group_rules (
    rule_id          TEXT PRIMARY KEY,
    secgrp_id       TEXT NOT NULL,
    direction       TEXT NOT NULL,
    ethertype       TEXT NOT NULL,
    protocol        TEXT,
    port_range_min  INTEGER,
    port_range_max  INTEGER,
    remote_ip_prefix TEXT,
    remote_group_id TEXT,
    FOREIGN KEY (secgrp_id) REFERENCES os_security_groups(secgrp_id) ON DELETE CASCADE
);

-- Insert dummy data into projects
INSERT INTO os_project_names (project_id, project_name) VALUES 
    ('proj-1', 'hc_alpha_project'),
    ('proj-2', 'hc_beta_project'),
    ('proj-3', 'hc_gamma_project'),
    ('proj-4', 'hc_delta_project'),
    ('proj-5', 'hc_epsilon_project'),
    ('proj-6', 'hc_zeta_project'),
    ('proj-7', 'hc_eta_project');

-- Insert dummy data into servers with IPv4 addresses and new fields
INSERT INTO os_servers (server_id, server_name, project_id, ipv4_addr, status, image_id, image_name, flavor_id, flavor_name, metadata) VALUES
    ('srv-101', 'sa1x-server-p1', 'proj-1', '192.168.1.101', 'ACTIVE', 'img-001', 'ubuntu-22.04-lts', 'flv-medium', 'm1.medium', '{"environment":"production","team":"platform","cost_center":"CC-1234"}'),
    ('srv-102', 'sa1x-server-p2', 'proj-1', '192.168.1.102', 'ACTIVE', 'img-001', 'ubuntu-22.04-lts', 'flv-small', 'm1.small', '{"environment":"staging","team":"platform"}'),
    ('srv-103', 'sa1x-server-p3', 'proj-2', '192.168.2.103', 'ACTIVE', 'img-002', 'centos-8-stream', 'flv-large', 'm1.large', '{"environment":"production","application":"database","backup":"daily"}'),
    ('srv-104', 'sa1x-server-p4', 'proj-2', '192.168.2.104', 'SHUTOFF', 'img-002', 'centos-8-stream', 'flv-medium', 'm1.medium', '{"environment":"development"}'),
    ('srv-105', 'sa1x-server-p5', 'proj-3', '192.168.3.105', 'ACTIVE', 'img-003', 'debian-11', 'flv-small', 'm1.small', NULL),
    ('srv-106', 'sa1x-server-p6', 'proj-3', '192.168.3.106', 'ACTIVE', 'img-003', 'debian-11', 'flv-small', 'm1.small', '{"owner":"john.doe@example.com"}'),
    ('srv-107', 'sa1x-server-p7', 'proj-4', '192.168.4.107', 'ACTIVE', 'img-004', 'rocky-9', 'flv-xlarge', 'm1.xlarge', '{"environment":"production","tier":"high-performance","sla":"99.9"}'),
    ('srv-108', 'sa1x-server-p8', 'proj-4', '192.168.4.108', 'BUILD', 'img-001', 'ubuntu-22.04-lts', 'flv-medium', 'm1.medium', NULL),
    ('srv-109', 'sa1x-server-p9', 'proj-5', '192.168.5.109', 'ACTIVE', 'img-005', 'windows-2022', 'flv-large', 'm1.large', '{"os_type":"windows","license":"enterprise"}'),
    ('srv-110', 'sa1x-server-p10', 'proj-5', '192.168.5.110', 'ERROR', 'img-001', 'ubuntu-22.04-lts', 'flv-small', 'm1.small', '{"environment":"test"}'),
    ('srv-111', 'sa1x-server-p11', 'proj-6', '192.168.6.111', 'ACTIVE', 'img-001', 'ubuntu-22.04-lts', 'flv-medium', 'm1.medium', '{"managed_by":"terraform","version":"1.5"}'),
    ('srv-112', 'sa1x-server-p12', 'proj-6', '192.168.6.112', 'ACTIVE', 'img-002', 'centos-8-stream', 'flv-medium', 'm1.medium', NULL),
    ('srv-113', 'sa1x-server-p13', 'proj-7', '192.168.7.113', 'SUSPENDED', 'img-003', 'debian-11', 'flv-large', 'm1.large', '{"reason":"maintenance"}'),
    ('srv-114', 'sa1x-server-p14', 'proj-7', '192.168.7.114', 'ACTIVE', 'img-004', 'rocky-9', 'flv-small', 'm1.small', '{"project":"analytics","department":"data-science"}');

-- Insert dummy data into security groups (multi-tier architecture)
INSERT INTO os_security_groups (secgrp_id, secgrp_name, project_id) VALUES
    ('sg-1', 'default', 'proj-1'),
    ('sg-2', 'web-servers', 'proj-1'),
    ('sg-3', 'default', 'proj-2'),
    ('sg-4', 'database', 'proj-2'),
    ('sg-5', 'default', 'proj-3'),
    ('sg-6', 'load-balancer', 'proj-1'),
    ('sg-7', 'app-servers', 'proj-2');

-- Insert dummy data into security group rules (realistic multi-tier architecture)
INSERT INTO os_security_group_rules (rule_id, secgrp_id, direction, ethertype, protocol, port_range_min, port_range_max, remote_ip_prefix, remote_group_id) VALUES
    -- sg-1 (default): SSH from anywhere
    ('rule-1', 'sg-1', 'ingress', 'IPv4', 'tcp', 22, 22, '0.0.0.0/0', NULL),
    ('rule-2', 'sg-1', 'egress', 'IPv4', NULL, NULL, NULL, '0.0.0.0/0', NULL),

    -- sg-2 (web-servers): HTTP/HTTPS from load-balancer group, SSH from private network
    ('rule-3', 'sg-2', 'ingress', 'IPv4', 'tcp', 80, 80, NULL, 'sg-6'),
    ('rule-4', 'sg-2', 'ingress', 'IPv4', 'tcp', 443, 443, NULL, 'sg-6'),
    ('rule-5', 'sg-2', 'ingress', 'IPv4', 'tcp', 22, 22, '10.0.0.0/8', NULL),
    ('rule-6', 'sg-2', 'egress', 'IPv4', 'tcp', 3306, 3306, NULL, 'sg-4'),
    ('rule-7', 'sg-2', 'egress', 'IPv4', 'udp', 53, 53, '8.8.8.8/32', NULL),

    -- sg-3 (default): SSH from private network
    ('rule-8', 'sg-3', 'ingress', 'IPv4', 'tcp', 22, 22, '10.0.0.0/8', NULL),

    -- sg-4 (database): MySQL from web-servers, PostgreSQL from app-servers
    ('rule-9', 'sg-4', 'ingress', 'IPv4', 'tcp', 3306, 3306, NULL, 'sg-2'),
    ('rule-10', 'sg-4', 'ingress', 'IPv4', 'tcp', 5432, 5432, NULL, 'sg-7'),
    ('rule-11', 'sg-4', 'ingress', 'IPv4', 'tcp', 22, 22, '172.16.0.0/12', NULL),

    -- sg-5 (default): No rules (for testing empty security groups)

    -- sg-6 (load-balancer): HTTP/HTTPS from anywhere
    ('rule-12', 'sg-6', 'ingress', 'IPv4', 'tcp', 80, 80, '0.0.0.0/0', NULL),
    ('rule-13', 'sg-6', 'ingress', 'IPv4', 'tcp', 443, 443, '0.0.0.0/0', NULL),
    ('rule-14', 'sg-6', 'egress', 'IPv4', 'tcp', 80, 80, NULL, 'sg-2'),
    ('rule-15', 'sg-6', 'egress', 'IPv4', 'tcp', 443, 443, NULL, 'sg-2'),

    -- sg-7 (app-servers): Custom app ports from web-servers, database access
    ('rule-16', 'sg-7', 'ingress', 'IPv4', 'tcp', 8080, 8080, NULL, 'sg-2'),
    ('rule-17', 'sg-7', 'ingress', 'IPv4', 'tcp', 9000, 9000, NULL, 'sg-2'),
    ('rule-18', 'sg-7', 'egress', 'IPv4', 'tcp', 5432, 5432, NULL, 'sg-4'),
    ('rule-19', 'sg-7', 'egress', 'IPv4', 'icmp', NULL, NULL, '8.8.8.8/32', NULL);

-- Create table for volumes
CREATE TABLE os_volumes (
    volume_id    TEXT PRIMARY KEY,
    volume_name  TEXT NOT NULL,
    size_gb      INTEGER NOT NULL,
    volume_type  TEXT,
    project_id   TEXT,
    FOREIGN KEY (project_id) REFERENCES os_project_names(project_id) ON DELETE CASCADE
);

-- Create table for server-security group mappings
CREATE TABLE os_server_secgrps (
    server_id TEXT NOT NULL,
    secgrp_id TEXT NOT NULL,
    PRIMARY KEY (server_id, secgrp_id),
    FOREIGN KEY (server_id) REFERENCES os_servers(server_id) ON DELETE CASCADE,
    FOREIGN KEY (secgrp_id) REFERENCES os_security_groups(secgrp_id) ON DELETE CASCADE
);

-- Create table for server-volume mappings
CREATE TABLE os_server_volumes (
    server_id   TEXT NOT NULL,
    volume_id   TEXT NOT NULL,
    device_path TEXT NOT NULL,
    PRIMARY KEY (server_id, volume_id),
    FOREIGN KEY (server_id) REFERENCES os_servers(server_id) ON DELETE CASCADE,
    FOREIGN KEY (volume_id) REFERENCES os_volumes(volume_id) ON DELETE CASCADE
);

-- Insert sample volumes with various sizes and types
INSERT INTO os_volumes (volume_id, volume_name, size_gb, volume_type, project_id) VALUES
    ('vol-1', 'data-volume-1', 100, 'SSD', 'proj-1'),
    ('vol-2', 'backup-volume-1', 500, 'HDD', 'proj-1'),
    ('vol-3', 'db-volume-1', 250, 'SSD', 'proj-2'),
    ('vol-4', 'app-volume-1', 50, 'SSD', 'proj-2'),
    ('vol-5', 'log-volume-1', 200, 'HDD', 'proj-3'),
    ('vol-6', 'cache-volume-1', 100, 'SSD', 'proj-3'),
    ('vol-7', 'temp-volume-1', 50, NULL, 'proj-4');

-- Insert server-security group mappings (multiple SGs per server)
INSERT INTO os_server_secgrps (server_id, secgrp_id) VALUES
    ('srv-101', 'sg-1'),
    ('srv-101', 'sg-2'),
    ('srv-102', 'sg-1'),
    ('srv-102', 'sg-6'),
    ('srv-103', 'sg-3'),
    ('srv-103', 'sg-4'),
    ('srv-104', 'sg-3'),
    ('srv-104', 'sg-7'),
    ('srv-105', 'sg-5'),
    ('srv-106', 'sg-5'),
    ('srv-107', 'sg-1'),
    ('srv-108', 'sg-1');

-- Insert server-volume mappings
INSERT INTO os_server_volumes (server_id, volume_id, device_path) VALUES
    ('srv-101', 'vol-1', '/dev/vdb'),
    ('srv-101', 'vol-2', '/dev/vdc'),
    ('srv-103', 'vol-3', '/dev/vdb'),
    ('srv-104', 'vol-4', '/dev/vdb'),
    ('srv-105', 'vol-5', '/dev/vdb'),
    ('srv-105', 'vol-6', '/dev/vdc'),
    ('srv-107', 'vol-7', '/dev/vdb');
