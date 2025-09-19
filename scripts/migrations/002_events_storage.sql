-- ============================================================================
-- OVIM Database Migration: 002 - Event Storage System
-- ============================================================================
--
-- This migration adds persistent event storage capabilities to OVIM:
--
-- 1. Creates events table for persistent event storage
-- 2. Adds indexes for efficient event querying and filtering
-- 3. Supports event categories, bulk operations, and search
-- 4. Integrates with existing Kubernetes event system
--
-- ============================================================================

-- Begin transaction
BEGIN;

-- ============================================================================
-- 1. CREATE EVENTS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Event identification
    name VARCHAR(255) NOT NULL,                    -- Event name for deduplication
    event_uid VARCHAR(255) UNIQUE,                 -- Kubernetes event UID (if applicable)

    -- Event classification
    type VARCHAR(50) NOT NULL DEFAULT 'Normal',    -- Normal, Warning, Error
    reason VARCHAR(255) NOT NULL,                  -- Event reason code
    category VARCHAR(100) DEFAULT 'General',       -- organization, vdc, vm, security, performance, integration
    component VARCHAR(100) NOT NULL,               -- ovim-api, ovim-controller, kubevirt, etc.

    -- Event content
    message TEXT NOT NULL,                         -- Human-readable event message
    action VARCHAR(255),                           -- Action that triggered the event

    -- Event context
    namespace VARCHAR(255),                        -- Kubernetes namespace
    org_id VARCHAR(255),                          -- Organization ID (if applicable)
    vdc_id VARCHAR(255),                          -- VDC ID (if applicable)
    vm_id VARCHAR(255),                           -- VM ID (if applicable)
    user_id VARCHAR(255),                         -- User who triggered the event
    username VARCHAR(255),                        -- Username for display

    -- Involved object (Kubernetes resource)
    involved_object_kind VARCHAR(100),             -- Organization, VDC, VM, Pod, etc.
    involved_object_name VARCHAR(255),             -- Name of the involved object
    involved_object_namespace VARCHAR(255),        -- Namespace of the involved object
    involved_object_uid VARCHAR(255),              -- UID of the involved object
    involved_object_resource_version VARCHAR(50),  -- Resource version

    -- Event metadata
    metadata JSONB DEFAULT '{}',                   -- Additional event metadata
    annotations JSONB DEFAULT '{}',                -- Event annotations
    labels JSONB DEFAULT '{}',                     -- Event labels for filtering

    -- Event timing
    first_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    event_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,  -- When the event actually occurred

    -- Event counting and aggregation
    count INTEGER DEFAULT 1,                       -- Number of times this event occurred

    -- Event source and reporting
    source_component VARCHAR(255),                 -- Component that generated the event
    source_host VARCHAR(255),                      -- Host where event was generated
    reporting_controller VARCHAR(255),             -- Controller that reported the event
    reporting_instance VARCHAR(255),               -- Instance ID of the reporting controller

    -- Event series (for related events)
    series_count INTEGER,                          -- Count in series
    series_last_observed_time TIMESTAMP,           -- Last time in series
    series_state VARCHAR(50),                      -- Series state

    -- Event lifecycle
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Soft delete for event retention
    deleted_at TIMESTAMP NULL,

    -- Constraints
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE SET NULL,
    FOREIGN KEY (vdc_id) REFERENCES virtual_data_centers(id) ON DELETE SET NULL,
    FOREIGN KEY (vm_id) REFERENCES virtual_machines(id) ON DELETE SET NULL
);

-- ============================================================================
-- 2. CREATE INDEXES FOR PERFORMANCE
-- ============================================================================

-- Primary query indexes
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_category ON events(category);
CREATE INDEX IF NOT EXISTS idx_events_reason ON events(reason);
CREATE INDEX IF NOT EXISTS idx_events_component ON events(component);
CREATE INDEX IF NOT EXISTS idx_events_namespace ON events(namespace);

-- Organizational indexes
CREATE INDEX IF NOT EXISTS idx_events_org_id ON events(org_id);
CREATE INDEX IF NOT EXISTS idx_events_vdc_id ON events(vdc_id);
CREATE INDEX IF NOT EXISTS idx_events_vm_id ON events(vm_id);
CREATE INDEX IF NOT EXISTS idx_events_user_id ON events(user_id);

-- Time-based indexes
CREATE INDEX IF NOT EXISTS idx_events_last_timestamp ON events(last_timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_first_timestamp ON events(first_timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_event_time ON events(event_time DESC);
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at DESC);

-- Object-based indexes
CREATE INDEX IF NOT EXISTS idx_events_involved_object ON events(involved_object_kind, involved_object_name);
CREATE INDEX IF NOT EXISTS idx_events_involved_object_namespace ON events(involved_object_namespace);

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_events_org_type_time ON events(org_id, type, last_timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_vdc_category_time ON events(vdc_id, category, last_timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_component_reason_time ON events(component, reason, last_timestamp DESC);

-- Full-text search index
CREATE INDEX IF NOT EXISTS idx_events_message_search ON events USING gin(to_tsvector('english', message));

-- Soft delete index
CREATE INDEX IF NOT EXISTS idx_events_deleted_at ON events(deleted_at);

-- ============================================================================
-- 3. CREATE EVENT CATEGORIES ENUM (for validation)
-- ============================================================================

-- Event categories for better organization
CREATE TABLE IF NOT EXISTS event_categories (
    name VARCHAR(100) PRIMARY KEY,
    description TEXT,
    color VARCHAR(20) DEFAULT '#1f77b4',  -- Color for UI display
    icon VARCHAR(50),                      -- Icon class for UI
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert default categories
INSERT INTO event_categories (name, description, color, icon) VALUES
('organization', 'Organization lifecycle and management events', '#2ca02c', 'fa-building'),
('vdc', 'Virtual Data Center resource and lifecycle events', '#ff7f0e', 'fa-server'),
('vm', 'Virtual Machine lifecycle and operation events', '#d62728', 'fa-desktop'),
('security', 'Authentication, authorization, and security events', '#9467bd', 'fa-shield-alt'),
('performance', 'Performance monitoring and optimization events', '#8c564b', 'fa-chart-line'),
('integration', 'External system integration and API events', '#e377c2', 'fa-plug'),
('system', 'System-level and infrastructure events', '#7f7f7f', 'fa-cogs'),
('audit', 'Audit trail and compliance events', '#bcbd22', 'fa-clipboard-list'),
('quota', 'Resource quota and limit events', '#17becf', 'fa-balance-scale'),
('network', 'Network configuration and connectivity events', '#aec7e8', 'fa-network-wired'),
('storage', 'Storage provisioning and management events', '#ffbb78', 'fa-hdd'),
('backup', 'Backup and disaster recovery events', '#98df8a', 'fa-archive')
ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    color = EXCLUDED.color,
    icon = EXCLUDED.icon;

-- ============================================================================
-- 4. CREATE EVENT RETENTION POLICY
-- ============================================================================

-- Event retention configuration
CREATE TABLE IF NOT EXISTS event_retention_policies (
    id SERIAL PRIMARY KEY,
    category VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL DEFAULT 'all',  -- Normal, Warning, Error, all
    retention_days INTEGER NOT NULL DEFAULT 30,
    max_events INTEGER DEFAULT 10000,         -- Max events before cleanup
    auto_cleanup BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(category, type)
);

-- Insert default retention policies
INSERT INTO event_retention_policies (category, type, retention_days, max_events) VALUES
('audit', 'all', 365, 100000),          -- Keep audit events for 1 year
('security', 'all', 180, 50000),        -- Keep security events for 6 months
('organization', 'all', 90, 10000),     -- Keep org events for 3 months
('vdc', 'all', 90, 20000),              -- Keep VDC events for 3 months
('vm', 'Warning', 60, 15000),           -- Keep VM warnings for 2 months
('vm', 'Error', 90, 5000),              -- Keep VM errors for 3 months
('vm', 'Normal', 30, 30000),            -- Keep VM normal events for 1 month
('performance', 'all', 30, 25000),      -- Keep performance events for 1 month
('integration', 'all', 60, 10000),      -- Keep integration events for 2 months
('system', 'Warning', 90, 5000),        -- Keep system warnings for 3 months
('system', 'Error', 180, 2000),         -- Keep system errors for 6 months
('system', 'Normal', 14, 10000)         -- Keep system normal events for 2 weeks
ON CONFLICT (category, type) DO UPDATE SET
    retention_days = EXCLUDED.retention_days,
    max_events = EXCLUDED.max_events;

-- ============================================================================
-- 5. CREATE UPDATED_AT TRIGGER
-- ============================================================================

-- Apply updated_at trigger to events table
DROP TRIGGER IF EXISTS update_events_updated_at ON events;
CREATE TRIGGER update_events_updated_at
    BEFORE UPDATE ON events
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Apply updated_at trigger to retention policies
DROP TRIGGER IF EXISTS update_event_retention_policies_updated_at ON event_retention_policies;
CREATE TRIGGER update_event_retention_policies_updated_at
    BEFORE UPDATE ON event_retention_policies
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- 6. CREATE EVENT CLEANUP FUNCTION
-- ============================================================================

-- Function to clean up old events based on retention policies
CREATE OR REPLACE FUNCTION cleanup_old_events()
RETURNS INTEGER AS $$
DECLARE
    policy RECORD;
    deleted_count INTEGER := 0;
    batch_deleted INTEGER;
BEGIN
    -- Loop through each retention policy
    FOR policy IN SELECT * FROM event_retention_policies WHERE auto_cleanup = TRUE LOOP
        -- Delete events older than retention period
        IF policy.type = 'all' THEN
            DELETE FROM events
            WHERE category = policy.category
            AND last_timestamp < NOW() - INTERVAL '1 day' * policy.retention_days
            AND deleted_at IS NULL;
        ELSE
            DELETE FROM events
            WHERE category = policy.category
            AND type = policy.type
            AND last_timestamp < NOW() - INTERVAL '1 day' * policy.retention_days
            AND deleted_at IS NULL;
        END IF;

        GET DIAGNOSTICS batch_deleted = ROW_COUNT;
        deleted_count := deleted_count + batch_deleted;

        -- If we still have too many events, delete oldest ones
        IF policy.type = 'all' THEN
            DELETE FROM events
            WHERE id IN (
                SELECT id FROM events
                WHERE category = policy.category
                AND deleted_at IS NULL
                ORDER BY last_timestamp DESC
                OFFSET policy.max_events
            );
        ELSE
            DELETE FROM events
            WHERE id IN (
                SELECT id FROM events
                WHERE category = policy.category
                AND type = policy.type
                AND deleted_at IS NULL
                ORDER BY last_timestamp DESC
                OFFSET policy.max_events
            );
        END IF;

        GET DIAGNOSTICS batch_deleted = ROW_COUNT;
        deleted_count := deleted_count + batch_deleted;
    END LOOP;

    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 7. COMMIT MIGRATION
-- ============================================================================

-- Record migration in schema_migrations table
INSERT INTO schema_migrations (version, description)
VALUES ('002', 'Event storage system with categories, retention, and search')
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
    RAISE NOTICE 'Migration 002 completed successfully!';
    RAISE NOTICE 'Summary of changes:';
    RAISE NOTICE '- Created events table with comprehensive event storage';
    RAISE NOTICE '- Added event_categories table with default categories';
    RAISE NOTICE '- Created event_retention_policies table with cleanup automation';
    RAISE NOTICE '- Added indexes for efficient event querying and search';
    RAISE NOTICE '- Created cleanup_old_events() function for retention management';
    RAISE NOTICE '- Added updated_at triggers for automatic timestamps';
    RAISE NOTICE '';
    RAISE NOTICE 'Event categories created:';
    RAISE NOTICE '- organization, vdc, vm, security, performance, integration';
    RAISE NOTICE '- system, audit, quota, network, storage, backup';
    RAISE NOTICE '';
    RAISE NOTICE 'Next steps:';
    RAISE NOTICE '1. Update EventRecorder to use database storage';
    RAISE NOTICE '2. Implement event API endpoints with filtering and search';
    RAISE NOTICE '3. Set up periodic cleanup job: SELECT cleanup_old_events();';
    RAISE NOTICE '4. Implement WebSocket support for real-time events';
END $$;