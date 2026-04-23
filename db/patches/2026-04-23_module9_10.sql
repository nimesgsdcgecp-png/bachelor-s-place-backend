-- =============================================================================
-- Patch: 2026-04-23 — Module 9 & 10 DB migrations
-- Run this ONCE against your live database.
-- Safe to run multiple times (uses IF NOT EXISTS / IF EXISTS guards).
-- =============================================================================

-- 1. Add move_in_confirmed_at to squads (Module 8 move-in endpoint)
ALTER TABLE squads ADD COLUMN IF NOT EXISTS move_in_confirmed_at TIMESTAMPTZ;

-- 2. Add notification_type ENUM (Module 10)
DO $$ BEGIN
    CREATE TYPE notification_type AS ENUM (
        'squad_invite',
        'squad_invite_accepted',
        'squad_invite_rejected',
        'squad_disbanded',
        'property_proposal',
        'proposal_accepted',
        'proposal_rejected',
        'kyc_approved',
        'kyc_rejected',
        'property_verified',
        'property_rejected',
        'token_payment_success',
        'move_in_confirmed'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL; -- already exists, skip
END $$;

-- 3. Create notifications table (Module 10)
CREATE TABLE IF NOT EXISTS notifications (
    id           UUID               PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID               NOT NULL REFERENCES users(id),
    type         notification_type  NOT NULL,
    title        TEXT               NOT NULL,
    body         TEXT               NOT NULL,
    is_read      BOOLEAN            NOT NULL DEFAULT FALSE,
    read_at      TIMESTAMPTZ,
    metadata     JSONB,
    created_at   TIMESTAMPTZ        NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

-- 4. Indexes for notifications
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
    ON notifications (user_id, created_at DESC)
    WHERE is_read = FALSE AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_notifications_user_id
    ON notifications (user_id);

CREATE INDEX IF NOT EXISTS idx_notifications_user_created_at
    ON notifications (user_id, created_at DESC)
    WHERE deleted_at IS NULL;
