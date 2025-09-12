-- ============================================================================
-- OVIM Database Migration: 001 - Organization and VDC CRD Architecture
-- ============================================================================
-- 
-- This migration transforms the OVIM database to support the new CRD-based
-- organization and virtual data center architecture:
-- 
-- 1. Organizations become identity/catalog containers (no resource quotas)
-- 2. VDCs become resource containers with quotas and workload isolation
-- 3. Add CRD integration fields for Kubernetes synchronization
-- 4. Support for Catalog CRDs and content management
--
-- ============================================================================

-- Begin transaction
BEGIN;

-- ============================================================================
-- 1. UPDATE ORGANIZATIONS TABLE (Remove quotas, add CRD fields)
-- ============================================================================

-- Remove quota fields from organizations (they move to VDCs)
-- No backward compatibility needed
ALTER TABLE organizations DROP COLUMN IF EXISTS cpu_quota;
ALTER TABLE organizations DROP COLUMN IF EXISTS memory_quota;
ALTER TABLE organizations DROP COLUMN IF EXISTS storage_quota;

-- Add new fields for CRD integration
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS display_name VARCHAR(255);
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS cr_name VARCHAR(255) UNIQUE;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS cr_namespace VARCHAR(255) DEFAULT 'default';
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS vdc_count INTEGER DEFAULT 0;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS last_rbac_sync TIMESTAMP NULL;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS observed_generation BIGINT DEFAULT 0;

-- Update existing organizations to have proper CR names
UPDATE organizations 
SET cr_name = name, 
    display_name = COALESCE(display_name, name)
WHERE cr_name IS NULL;

-- ============================================================================
-- 2. CREATE VIRTUAL_DATA_CENTERS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS virtual_data_centers (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    org_id VARCHAR(255) NOT NULL,
    
    -- CRD integration
    display_name VARCHAR(255),
    cr_name VARCHAR(255) NOT NULL,        -- CRD instance name
    cr_namespace VARCHAR(255) NOT NULL,   -- Org namespace where VDC CR lives
    workload_namespace VARCHAR(255) UNIQUE NOT NULL,  -- vdc-<org>-<vdc>
    
    -- Resource quotas (stored as integers for easier calculation)
    cpu_quota INTEGER NOT NULL DEFAULT 0,
    memory_quota INTEGER NOT NULL DEFAULT 0,  -- in GB
    storage_quota INTEGER NOT NULL DEFAULT 0, -- in GB
    pods_quota INTEGER NOT NULL DEFAULT 100,
    vms_quota INTEGER NOT NULL DEFAULT 50,
    
    -- VM LimitRange (optional, in millicores and MiB)
    min_cpu INTEGER NULL,    -- millicores
    max_cpu INTEGER NULL,    -- millicores  
    min_memory INTEGER NULL, -- MiB
    max_memory INTEGER NULL, -- MiB
    
    -- Network and configuration
    network_policy VARCHAR(100) DEFAULT 'default',
    custom_network_config JSONB NULL,
    catalog_restrictions JSONB NULL,
    
    -- Status tracking
    phase VARCHAR(50) DEFAULT 'Pending',
    conditions JSONB NULL,
    observed_generation BIGINT DEFAULT 0,
    
    -- Resource usage tracking (updated by metrics controller)
    cpu_used INTEGER DEFAULT 0,
    memory_used INTEGER DEFAULT 0,
    storage_used INTEGER DEFAULT 0,
    cpu_percentage DECIMAL(5,2) DEFAULT 0.0,
    memory_percentage DECIMAL(5,2) DEFAULT 0.0,
    storage_percentage DECIMAL(5,2) DEFAULT 0.0,
    
    -- Workload counts
    total_pods INTEGER DEFAULT 0,
    running_pods INTEGER DEFAULT 0,
    total_vms INTEGER DEFAULT 0,
    running_vms INTEGER DEFAULT 0,
    
    -- Sync tracking
    last_metrics_update TIMESTAMP NULL,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints and indexes
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
    UNIQUE (org_id, name)  -- VDC names must be unique within organization
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_vdc_org_id ON virtual_data_centers(org_id);
CREATE INDEX IF NOT EXISTS idx_vdc_workload_namespace ON virtual_data_centers(workload_namespace);
CREATE INDEX IF NOT EXISTS idx_vdc_cr_name ON virtual_data_centers(cr_name);
CREATE INDEX IF NOT EXISTS idx_vdc_cr_namespace ON virtual_data_centers(cr_namespace);
CREATE INDEX IF NOT EXISTS idx_vdc_phase ON virtual_data_centers(phase);

-- ============================================================================
-- 3. CREATE CATALOGS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS catalogs (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    org_id VARCHAR(255) NOT NULL,
    
    -- CRD integration
    display_name VARCHAR(255),
    cr_name VARCHAR(255) NOT NULL,
    cr_namespace VARCHAR(255) NOT NULL,
    
    -- Catalog configuration
    type VARCHAR(50) DEFAULT 'vm-template', -- vm-template, application-stack, mixed
    source_type VARCHAR(50) NOT NULL,       -- git, oci, s3, http, local
    source_url TEXT NOT NULL,
    source_branch VARCHAR(255) DEFAULT 'main',
    source_path VARCHAR(1000) DEFAULT '/',
    source_credentials VARCHAR(255) NULL,   -- Secret name
    insecure_skip_tls_verify BOOLEAN DEFAULT FALSE,
    refresh_interval VARCHAR(50) DEFAULT '1h',
    
    -- Content filtering
    include_patterns JSONB NULL,
    exclude_patterns JSONB NULL,
    required_tags JSONB NULL,
    
    -- Permissions
    allowed_vdcs JSONB NULL,
    allowed_groups JSONB NULL,
    read_only BOOLEAN DEFAULT TRUE,
    
    -- Status
    is_enabled BOOLEAN DEFAULT TRUE,
    phase VARCHAR(50) DEFAULT 'Pending',
    conditions JSONB NULL,
    
    -- Content summary
    total_items INTEGER DEFAULT 0,
    vm_templates INTEGER DEFAULT 0,
    application_stacks INTEGER DEFAULT 0,
    categories JSONB NULL,
    
    -- Sync status
    last_sync TIMESTAMP NULL,
    last_sync_attempt TIMESTAMP NULL,
    sync_errors JSONB NULL,
    next_sync_scheduled TIMESTAMP NULL,
    
    observed_generation BIGINT DEFAULT 0,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints and indexes
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
    UNIQUE (org_id, name)  -- Catalog names must be unique within organization
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_catalog_org_id ON catalogs(org_id);
CREATE INDEX IF NOT EXISTS idx_catalog_cr_name ON catalogs(cr_name);
CREATE INDEX IF NOT EXISTS idx_catalog_cr_namespace ON catalogs(cr_namespace);
CREATE INDEX IF NOT EXISTS idx_catalog_type ON catalogs(type);
CREATE INDEX IF NOT EXISTS idx_catalog_phase ON catalogs(phase);
CREATE INDEX IF NOT EXISTS idx_catalog_is_enabled ON catalogs(is_enabled);

-- ============================================================================
-- 4. UPDATE VIRTUAL_MACHINES TABLE (Add VDC association)
-- ============================================================================

-- Add VDC association to virtual machines
ALTER TABLE virtual_machines ADD COLUMN IF NOT EXISTS vdc_id VARCHAR(255) NULL;
CREATE INDEX IF NOT EXISTS idx_vm_vdc_id ON virtual_machines(vdc_id);

-- ============================================================================
-- 5. UPDATE TEMPLATES TABLE (Add catalog integration)
-- ============================================================================

-- Add catalog integration to templates
ALTER TABLE templates ADD COLUMN IF NOT EXISTS catalog_id VARCHAR(255) NULL;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS content_type VARCHAR(50) DEFAULT 'vm-template';
CREATE INDEX IF NOT EXISTS idx_template_catalog_id ON templates(catalog_id);

-- ============================================================================
-- 6. CREATE UPDATED_AT TRIGGERS
-- ============================================================================

-- Function to update updated_at column
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE 'plpgsql';

-- Apply triggers to tables
DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;
CREATE TRIGGER update_organizations_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_virtual_data_centers_updated_at ON virtual_data_centers;
CREATE TRIGGER update_virtual_data_centers_updated_at
    BEFORE UPDATE ON virtual_data_centers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_catalogs_updated_at ON catalogs;
CREATE TRIGGER update_catalogs_updated_at
    BEFORE UPDATE ON catalogs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 7. DATA CONSISTENCY CHECKS
-- ============================================================================

-- Ensure all organizations have proper namespaces
UPDATE organizations 
SET namespace = 'org-' || name 
WHERE namespace IS NULL OR namespace = '';

-- Ensure all organizations have CR names
UPDATE organizations 
SET cr_name = name 
WHERE cr_name IS NULL;

-- ============================================================================
-- 8. COMMIT MIGRATION
-- ============================================================================

-- Record migration in schema_migrations table (create if not exists)
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(50) PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);

-- Record this migration
INSERT INTO schema_migrations (version, description) 
VALUES ('001', 'Organization and VDC CRD architecture migration')
ON CONFLICT (version) DO UPDATE SET 
    applied_at = CURRENT_TIMESTAMP,
    description = EXCLUDED.description;

-- Commit transaction
COMMIT;

-- ============================================================================
-- MIGRATION COMPLETE
-- ============================================================================

-- Display migration summary
DO $$ 
BEGIN
    RAISE NOTICE 'Migration 001 completed successfully!';
    RAISE NOTICE 'Summary of changes:';
    RAISE NOTICE '- Updated organizations table (removed quotas, added CRD fields)';
    RAISE NOTICE '- Created virtual_data_centers table with resource quotas';
    RAISE NOTICE '- Created catalogs table for content management';
    RAISE NOTICE '- Updated virtual_machines table with VDC association';
    RAISE NOTICE '- Updated templates table with catalog integration';
    RAISE NOTICE '- Added updated_at triggers for automatic timestamps';
    RAISE NOTICE '- Created indexes for optimal query performance';
    RAISE NOTICE '';
    RAISE NOTICE 'Next steps:';
    RAISE NOTICE '1. Deploy CRD definitions: kubectl apply -f config/crd/';
    RAISE NOTICE '2. Deploy controllers to sync CRD status with database';
    RAISE NOTICE '3. Validate migration with: psql -f scripts/migrations/validate_migration_001.sql';
END $$;