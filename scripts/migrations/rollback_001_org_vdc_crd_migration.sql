-- ============================================================================
-- OVIM Database Rollback: 001 - Organization and VDC CRD Architecture
-- ============================================================================
-- 
-- WARNING: This rollback script will DESTROY VDC and Catalog data!
-- This is a destructive operation that cannot be undone.
-- 
-- This script rolls back the CRD architecture migration and restores
-- the legacy organization-quota model.
--
-- ============================================================================

-- Begin transaction
BEGIN;

\echo 'WARNING: This will destroy all VDC and Catalog data!'
\echo 'Press Ctrl+C to cancel, or continue to proceed with rollback...'

-- ============================================================================
-- 1. DROP NEW TABLES (WARNING: DATA LOSS)
-- ============================================================================

-- Drop catalogs table and all data
DROP TABLE IF EXISTS catalogs CASCADE;

-- Drop virtual_data_centers table and all data
DROP TABLE IF EXISTS virtual_data_centers CASCADE;

-- ============================================================================
-- 2. RESTORE ORGANIZATIONS TABLE (Remove CRD fields, add quotas back)
-- ============================================================================

-- Remove CRD integration fields
ALTER TABLE organizations DROP COLUMN IF EXISTS display_name;
ALTER TABLE organizations DROP COLUMN IF EXISTS cr_name;
ALTER TABLE organizations DROP COLUMN IF EXISTS cr_namespace;
ALTER TABLE organizations DROP COLUMN IF EXISTS vdc_count;
ALTER TABLE organizations DROP COLUMN IF EXISTS last_rbac_sync;
ALTER TABLE organizations DROP COLUMN IF EXISTS observed_generation;

-- Restore quota fields to organizations
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS cpu_quota INTEGER DEFAULT 0;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS memory_quota INTEGER DEFAULT 0;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS storage_quota INTEGER DEFAULT 0;

-- ============================================================================
-- 3. CLEAN UP VIRTUAL_MACHINES TABLE
-- ============================================================================

-- Remove VDC association from virtual machines
ALTER TABLE virtual_machines DROP COLUMN IF EXISTS vdc_id;

-- ============================================================================
-- 4. CLEAN UP TEMPLATES TABLE
-- ============================================================================

-- Remove catalog integration from templates
ALTER TABLE templates DROP COLUMN IF EXISTS catalog_id;
ALTER TABLE templates DROP COLUMN IF EXISTS content_type;

-- ============================================================================
-- 5. DROP TRIGGERS
-- ============================================================================

DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;
DROP TRIGGER IF EXISTS update_virtual_data_centers_updated_at ON virtual_data_centers;
DROP TRIGGER IF EXISTS update_catalogs_updated_at ON catalogs;

-- ============================================================================
-- 6. REMOVE MIGRATION RECORD
-- ============================================================================

DELETE FROM schema_migrations WHERE version = '001';

-- ============================================================================
-- 7. COMMIT ROLLBACK
-- ============================================================================

COMMIT;

\echo 'Rollback 001 completed - CRD architecture has been removed.'
\echo 'All VDC and Catalog data has been permanently deleted.'
\echo 'Organizations have been restored to legacy quota model.'