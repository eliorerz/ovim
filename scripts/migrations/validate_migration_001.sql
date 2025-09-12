-- ============================================================================
-- OVIM Migration Validation: 001 - Organization and VDC CRD Architecture
-- ============================================================================
-- 
-- This script validates that migration 001 was applied correctly and the
-- database schema matches the expected CRD-based architecture.
--
-- Run with: psql -d ovim -f scripts/migrations/validate_migration_001.sql
--
-- ============================================================================

\echo '============================================================================'
\echo 'OVIM Migration 001 Validation Report'
\echo '============================================================================'
\echo ''

-- Check if migration was recorded
\echo '1. MIGRATION RECORD CHECK'
\echo '-------------------------'
SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM schema_migrations WHERE version = '001') 
        THEN '✅ Migration 001 is recorded in schema_migrations'
        ELSE '❌ Migration 001 is NOT recorded - migration may have failed'
    END as migration_status;

SELECT version, applied_at, description 
FROM schema_migrations 
WHERE version = '001';

\echo ''

-- Check organizations table structure
\echo '2. ORGANIZATIONS TABLE VALIDATION'
\echo '---------------------------------'

-- Check that quota columns were removed
SELECT 
    CASE 
        WHEN NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'organizations' AND column_name = 'cpu_quota')
        THEN '✅ cpu_quota column removed from organizations'
        ELSE '❌ cpu_quota column still exists in organizations'
    END as cpu_quota_check;

SELECT 
    CASE 
        WHEN NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'organizations' AND column_name = 'memory_quota')
        THEN '✅ memory_quota column removed from organizations'
        ELSE '❌ memory_quota column still exists in organizations'
    END as memory_quota_check;

SELECT 
    CASE 
        WHEN NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'organizations' AND column_name = 'storage_quota')
        THEN '✅ storage_quota column removed from organizations'
        ELSE '❌ storage_quota column still exists in organizations'
    END as storage_quota_check;

-- Check that CRD fields were added
SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'organizations' AND column_name = 'display_name')
        THEN '✅ display_name column added to organizations'
        ELSE '❌ display_name column missing from organizations'
    END as display_name_check;

SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'organizations' AND column_name = 'cr_name')
        THEN '✅ cr_name column added to organizations'
        ELSE '❌ cr_name column missing from organizations'
    END as cr_name_check;

SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'organizations' AND column_name = 'vdc_count')
        THEN '✅ vdc_count column added to organizations'
        ELSE '❌ vdc_count column missing from organizations'
    END as vdc_count_check;

\echo ''

-- Check virtual_data_centers table
\echo '3. VIRTUAL_DATA_CENTERS TABLE VALIDATION'
\echo '----------------------------------------'

SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'virtual_data_centers')
        THEN '✅ virtual_data_centers table created'
        ELSE '❌ virtual_data_centers table missing'
    END as vdc_table_check;

-- Check key VDC columns
SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'virtual_data_centers' AND column_name = 'workload_namespace')
        THEN '✅ workload_namespace column exists in virtual_data_centers'
        ELSE '❌ workload_namespace column missing from virtual_data_centers'
    END as vdc_workload_namespace_check;

SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'virtual_data_centers' AND column_name = 'cpu_quota')
        THEN '✅ cpu_quota column exists in virtual_data_centers'
        ELSE '❌ cpu_quota column missing from virtual_data_centers'
    END as vdc_cpu_quota_check;

SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'virtual_data_centers' AND column_name = 'conditions')
        THEN '✅ conditions JSONB column exists in virtual_data_centers'
        ELSE '❌ conditions JSONB column missing from virtual_data_centers'
    END as vdc_conditions_check;

\echo ''

-- Check catalogs table
\echo '4. CATALOGS TABLE VALIDATION'
\echo '----------------------------'

SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'catalogs')
        THEN '✅ catalogs table created'
        ELSE '❌ catalogs table missing'
    END as catalogs_table_check;

-- Check key catalog columns
SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'catalogs' AND column_name = 'source_type')
        THEN '✅ source_type column exists in catalogs'
        ELSE '❌ source_type column missing from catalogs'
    END as catalog_source_type_check;

SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'catalogs' AND column_name = 'sync_errors')
        THEN '✅ sync_errors JSONB column exists in catalogs'
        ELSE '❌ sync_errors JSONB column missing from catalogs'
    END as catalog_sync_errors_check;

\echo ''

-- Check virtual_machines table updates
\echo '5. VIRTUAL_MACHINES TABLE VALIDATION'
\echo '------------------------------------'

SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'virtual_machines' AND column_name = 'vdc_id')
        THEN '✅ vdc_id column added to virtual_machines'
        ELSE '❌ vdc_id column missing from virtual_machines'
    END as vm_vdc_id_check;

\echo ''

-- Check templates table updates
\echo '6. TEMPLATES TABLE VALIDATION'
\echo '-----------------------------'

SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'templates' AND column_name = 'catalog_id')
        THEN '✅ catalog_id column added to templates'
        ELSE '❌ catalog_id column missing from templates'
    END as template_catalog_id_check;

SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'templates' AND column_name = 'content_type')
        THEN '✅ content_type column added to templates'
        ELSE '❌ content_type column missing from templates'
    END as template_content_type_check;

\echo ''

-- Check indexes
\echo '7. INDEX VALIDATION'
\echo '-------------------'

-- Count important indexes
SELECT 
    COUNT(*) as vdc_indexes_count,
    CASE 
        WHEN COUNT(*) >= 5 
        THEN '✅ VDC indexes created'
        ELSE '❌ Some VDC indexes missing'
    END as vdc_indexes_status
FROM pg_indexes 
WHERE tablename = 'virtual_data_centers' 
AND indexname LIKE 'idx_%';

SELECT 
    COUNT(*) as catalog_indexes_count,
    CASE 
        WHEN COUNT(*) >= 5 
        THEN '✅ Catalog indexes created'
        ELSE '❌ Some catalog indexes missing'
    END as catalog_indexes_status
FROM pg_indexes 
WHERE tablename = 'catalogs' 
AND indexname LIKE 'idx_%';

\echo ''

-- Check triggers
\echo '8. TRIGGER VALIDATION'
\echo '--------------------'

SELECT 
    COUNT(*) as trigger_count,
    CASE 
        WHEN COUNT(*) >= 3 
        THEN '✅ Updated_at triggers created'
        ELSE '❌ Some updated_at triggers missing'
    END as triggers_status
FROM information_schema.triggers 
WHERE trigger_name LIKE '%updated_at%';

\echo ''

-- Check data consistency
\echo '9. DATA CONSISTENCY VALIDATION'
\echo '------------------------------'

-- Check organizations have proper CR names
SELECT 
    COUNT(*) as orgs_without_cr_name,
    CASE 
        WHEN COUNT(*) = 0 
        THEN '✅ All organizations have CR names'
        ELSE '❌ Some organizations missing CR names'
    END as org_cr_name_status
FROM organizations 
WHERE cr_name IS NULL OR cr_name = '';

-- Check organizations have proper namespaces
SELECT 
    COUNT(*) as orgs_without_namespace,
    CASE 
        WHEN COUNT(*) = 0 
        THEN '✅ All organizations have namespaces'
        ELSE '❌ Some organizations missing namespaces'
    END as org_namespace_status
FROM organizations 
WHERE namespace IS NULL OR namespace = '';

\echo ''

-- Check foreign key constraints
\echo '10. FOREIGN KEY VALIDATION'
\echo '-------------------------'

SELECT 
    COUNT(*) as vdc_fk_count,
    CASE 
        WHEN COUNT(*) >= 1 
        THEN '✅ VDC foreign key constraints exist'
        ELSE '❌ VDC foreign key constraints missing'
    END as vdc_fk_status
FROM information_schema.table_constraints 
WHERE table_name = 'virtual_data_centers' 
AND constraint_type = 'FOREIGN KEY';

SELECT 
    COUNT(*) as catalog_fk_count,
    CASE 
        WHEN COUNT(*) >= 1 
        THEN '✅ Catalog foreign key constraints exist'
        ELSE '❌ Catalog foreign key constraints missing'
    END as catalog_fk_status
FROM information_schema.table_constraints 
WHERE table_name = 'catalogs' 
AND constraint_type = 'FOREIGN KEY';

\echo ''

-- Summary statistics
\echo '11. SUMMARY STATISTICS'
\echo '---------------------'

SELECT 
    (SELECT COUNT(*) FROM organizations) as total_organizations,
    (SELECT COUNT(*) FROM virtual_data_centers) as total_vdcs,
    (SELECT COUNT(*) FROM catalogs) as total_catalogs,
    (SELECT COUNT(*) FROM virtual_machines) as total_vms,
    (SELECT COUNT(*) FROM templates) as total_templates;

\echo ''
\echo '============================================================================'
\echo 'VALIDATION COMPLETE'
\echo '============================================================================'
\echo ''
\echo 'If all checks show ✅, migration 001 was successful!'
\echo 'If any checks show ❌, please review the migration script and rerun if needed.'
\echo ''
\echo 'Next steps after successful validation:'
\echo '1. Deploy CRDs: kubectl apply -f config/crd/'
\echo '2. Deploy controllers to manage CRD lifecycle'
\echo '3. Test API endpoints with new VDC functionality'
\echo ''