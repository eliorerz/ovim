-- OVIM Database Initialization Script
-- This script creates extensions and initial database setup

-- Enable UUID extension for generating UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Enable pgcrypto for password hashing (if needed)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create initial schemas if needed
-- GORM will handle table creation via AutoMigrate

-- Set timezone
SET timezone = 'UTC';

-- Create indexes that GORM might not create automatically
-- These will be created after GORM migration in the application