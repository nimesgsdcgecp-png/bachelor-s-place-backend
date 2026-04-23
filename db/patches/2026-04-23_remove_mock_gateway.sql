-- =============================================================================
-- Patch: 2026-04-23 — Remove mock gateway from live data
-- Run this ONCE against your live database.
-- PostgreSQL does not support DROP VALUE from an ENUM, but we clean up data
-- and prevent new rows from using 'mock' via application code.
-- =============================================================================

-- Update any test transactions that used the mock gateway to razorpay
-- (Safe to run multiple times — idempotent)
UPDATE transactions
SET gateway = 'razorpay'
WHERE gateway = 'mock';

-- Note: The 'mock' enum value cannot be dropped from PostgreSQL ENUM types.
-- It will remain in the type definition but will never be written by the application.
-- Fresh installs (using schema_initializer.sql) will not include 'mock'.
