-- OVIM Zones Integration Migration
-- Adds support for ACM cluster zones for VDC deployment targeting

-- Create zones table to store ACM managed clusters
CREATE TABLE IF NOT EXISTS zones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    cluster_name VARCHAR(255) NOT NULL,
    api_url VARCHAR(500) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'available',
    region VARCHAR(100),
    cloud_provider VARCHAR(50),

    -- Physical cluster capacity
    node_count INTEGER DEFAULT 0,
    cpu_capacity INTEGER DEFAULT 0,      -- Total CPU cores available
    memory_capacity INTEGER DEFAULT 0,   -- Total memory in GB
    storage_capacity INTEGER DEFAULT 0,  -- Total storage in GB

    -- Zone-level quotas (can be allocated to organizations)
    cpu_quota INTEGER DEFAULT 0,         -- CPU cores allocated to orgs
    memory_quota INTEGER DEFAULT 0,      -- Memory in GB allocated to orgs
    storage_quota INTEGER DEFAULT 0,     -- Storage in GB allocated to orgs

    -- Metadata from ACM
    labels JSONB DEFAULT '{}',
    annotations JSONB DEFAULT '{}',

    -- Sync tracking
    last_sync TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_zones_name ON zones(name);
CREATE INDEX IF NOT EXISTS idx_zones_status ON zones(status);
CREATE INDEX IF NOT EXISTS idx_zones_cluster_name ON zones(cluster_name);
CREATE INDEX IF NOT EXISTS idx_zones_last_sync ON zones(last_sync);

-- Organization-Zone quotas for access control and resource allocation
CREATE TABLE IF NOT EXISTS organization_zone_quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(255) NOT NULL,
    zone_id UUID NOT NULL,

    -- Resource quotas for this org in this zone
    cpu_quota INTEGER NOT NULL DEFAULT 0,
    memory_quota INTEGER NOT NULL DEFAULT 0,
    storage_quota INTEGER NOT NULL DEFAULT 0,

    -- Access control
    is_allowed BOOLEAN NOT NULL DEFAULT true,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    FOREIGN KEY (zone_id) REFERENCES zones(id) ON DELETE CASCADE,
    UNIQUE(organization_id, zone_id)
);

-- Add indexes for organization-zone lookups
CREATE INDEX IF NOT EXISTS idx_org_zone_quotas_org ON organization_zone_quotas(organization_id);
CREATE INDEX IF NOT EXISTS idx_org_zone_quotas_zone ON organization_zone_quotas(zone_id);
CREATE INDEX IF NOT EXISTS idx_org_zone_quotas_allowed ON organization_zone_quotas(is_allowed);

-- Add zone reference to VDCs table
-- Note: Making this nullable initially for backward compatibility
ALTER TABLE virtual_data_centers ADD COLUMN IF NOT EXISTS zone_id UUID;

-- Add foreign key constraint
ALTER TABLE virtual_data_centers
ADD CONSTRAINT IF NOT EXISTS fk_vdc_zone
FOREIGN KEY (zone_id) REFERENCES zones(id);

-- Add index for VDC zone lookups
CREATE INDEX IF NOT EXISTS idx_vdc_zone ON virtual_data_centers(zone_id);

-- Insert default local zone for existing deployments
INSERT INTO zones (
    name,
    cluster_name,
    api_url,
    status,
    region,
    cloud_provider,
    cpu_capacity,
    memory_capacity,
    storage_capacity,
    cpu_quota,
    memory_quota,
    storage_quota,
    labels,
    annotations
) VALUES (
    'local-cluster',
    'local-cluster',
    'https://kubernetes.default.svc',
    'available',
    'local',
    'local',
    100,  -- Default capacity values
    256,  -- 256 GB memory
    1000, -- 1TB storage
    80,   -- 80% of capacity allocated as quota
    200,  -- 200 GB memory quota
    800,  -- 800 GB storage quota
    '{"zone.ovim.io/type": "local", "zone.ovim.io/default": "true"}',
    '{"zone.ovim.io/description": "Default local cluster zone"}'
) ON CONFLICT (name) DO NOTHING;

-- Update existing VDCs to use the default local zone
UPDATE virtual_data_centers
SET zone_id = (SELECT id FROM zones WHERE name = 'local-cluster')
WHERE zone_id IS NULL;

-- Create view for zone utilization summary
CREATE OR REPLACE VIEW zone_utilization AS
SELECT
    z.id,
    z.name,
    z.status,
    z.cpu_capacity,
    z.memory_capacity,
    z.storage_capacity,
    z.cpu_quota,
    z.memory_quota,
    z.storage_quota,
    COALESCE(SUM(vdc.cpu_quota), 0) as cpu_used,
    COALESCE(SUM(vdc.memory_quota), 0) as memory_used,
    COALESCE(SUM(vdc.storage_quota), 0) as storage_used,
    COUNT(vdc.id) as vdc_count,
    COUNT(CASE WHEN vdc.phase = 'Active' THEN 1 END) as active_vdc_count,
    z.last_sync,
    z.updated_at
FROM zones z
LEFT JOIN virtual_data_centers vdc ON z.id = vdc.zone_id
GROUP BY z.id, z.name, z.status, z.cpu_capacity, z.memory_capacity,
         z.storage_capacity, z.cpu_quota, z.memory_quota, z.storage_quota,
         z.last_sync, z.updated_at;

-- Create view for organization zone access
CREATE OR REPLACE VIEW organization_zone_access AS
SELECT
    ozq.organization_id,
    z.id as zone_id,
    z.name as zone_name,
    z.status as zone_status,
    ozq.cpu_quota,
    ozq.memory_quota,
    ozq.storage_quota,
    ozq.is_allowed,
    COALESCE(SUM(vdc.cpu_quota), 0) as cpu_used,
    COALESCE(SUM(vdc.memory_quota), 0) as memory_used,
    COALESCE(SUM(vdc.storage_quota), 0) as storage_used,
    COUNT(vdc.id) as vdc_count
FROM organization_zone_quotas ozq
JOIN zones z ON ozq.zone_id = z.id
LEFT JOIN virtual_data_centers vdc ON z.id = vdc.zone_id AND vdc.org_id = ozq.organization_id
GROUP BY ozq.organization_id, z.id, z.name, z.status, ozq.cpu_quota,
         ozq.memory_quota, ozq.storage_quota, ozq.is_allowed;

-- Add comments for documentation
COMMENT ON TABLE zones IS 'ACM managed clusters available as deployment zones';
COMMENT ON TABLE organization_zone_quotas IS 'Resource quotas and access control for organizations within zones';
COMMENT ON COLUMN zones.status IS 'Zone status: available, unavailable, maintenance';
COMMENT ON COLUMN zones.cpu_capacity IS 'Total CPU cores available in the zone';
COMMENT ON COLUMN zones.memory_capacity IS 'Total memory in GB available in the zone';
COMMENT ON COLUMN zones.storage_capacity IS 'Total storage in GB available in the zone';
COMMENT ON COLUMN zones.cpu_quota IS 'CPU cores allocated to organizations';
COMMENT ON COLUMN zones.memory_quota IS 'Memory in GB allocated to organizations';
COMMENT ON COLUMN zones.storage_quota IS 'Storage in GB allocated to organizations';
COMMENT ON COLUMN virtual_data_centers.zone_id IS 'Zone where this VDC is deployed (immutable after creation)';