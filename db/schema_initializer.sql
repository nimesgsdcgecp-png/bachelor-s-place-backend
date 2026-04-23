-- =============================================================================
-- BachelorPad — Schema Initializer
-- Version: 1.1 (Consolidated)
-- Date: April 2026
--
-- Run this file ONCE on a fresh PostgreSQL 16+ database.
-- Prerequisites: PostGIS 3.4+ and pgvector 0.7+ must be installed on the server.
-- =============================================================================

-- =============================================================================
-- 1. EXTENSIONS
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS "pgcrypto";   -- gen_random_uuid(), pgp_sym_encrypt
CREATE EXTENSION IF NOT EXISTS postgis;       -- GEOGRAPHY type, spatial functions
CREATE EXTENSION IF NOT EXISTS vector;        -- pgvector for personality embeddings

-- =============================================================================
-- 2. ENUM TYPES
-- =============================================================================

CREATE TYPE user_role AS ENUM (
    'tenant',
    'landlord',
    'admin'
);

CREATE TYPE property_type AS ENUM (
    'room',
    'flat',
    'pg',
    'studio'
);

CREATE TYPE property_status AS ENUM (
    'draft',
    'pending_verification',
    'verified',
    'occupied',
    'delisted'
);

CREATE TYPE room_type AS ENUM (
    'single',
    'double',
    'triple',
    'dormitory'
);

CREATE TYPE squad_status AS ENUM (
    'browsing',     -- Squad-first flow: no property selected yet
    'forming',      -- Property identified; members finalizing
    'locked',       -- Token paid; property reserved
    'moved_in',     -- Move-in confirmed; payout triggered
    'disbanded'     -- Group dissolved
);

CREATE TYPE squad_member_role AS ENUM (
    'leader',
    'member'
);

CREATE TYPE squad_member_status AS ENUM (
    'invited',
    'accepted',
    'rejected',
    'left'
);

CREATE TYPE proposal_status AS ENUM (
    'pending',
    'accepted',
    'rejected'
);

CREATE TYPE verification_type AS ENUM (
    'ai_photo',     -- Automated stock image check
    'manual',       -- Admin physical/document review
    'virtual_tour', -- Admin-conducted virtual tour
    'physical'      -- Admin on-site visit
);

CREATE TYPE verification_status AS ENUM (
    'pending',
    'approved',
    'rejected'
);

CREATE TYPE transaction_type AS ENUM (
    'token_payment',  -- Tenant pays commitment fee (escrow)
    'success_fee',    -- Platform charges landlord on move-in
    'refund',         -- Returned to tenant
    'payout'          -- Sent to landlord
);

CREATE TYPE transaction_status AS ENUM (
    'initiated',  -- Record created before gateway call
    'success',
    'failed',
    'refunded'
);

CREATE TYPE kyc_status AS ENUM (
    'pending',
    'verified',
    'rejected'
);

CREATE TYPE message_content_type AS ENUM (
    'text',
    'system',   -- e.g. "User joined the squad"
    'file'
);

CREATE TYPE lookup_status AS ENUM (
    'active',
    'matched',
    'inactive'
);

CREATE TYPE payment_gateway AS ENUM (
    'razorpay',
    'stripe'
);

CREATE TYPE payment_model AS ENUM (
    'leader_pays_all',
    'split_evenly'
);

CREATE TYPE notification_type AS ENUM (
    'squad_invite',           -- Someone invited you to a squad
    'squad_invite_accepted',  -- A member accepted your squad invite
    'squad_invite_rejected',  -- A member rejected your squad invite
    'squad_disbanded',        -- Your squad was disbanded
    'property_proposal',      -- A squad member proposed a property
    'proposal_accepted',      -- Squad leader accepted a property proposal
    'proposal_rejected',      -- Squad leader rejected a property proposal
    'kyc_approved',           -- Your KYC was approved (landlord)
    'kyc_rejected',           -- Your KYC was rejected (landlord)
    'property_verified',      -- Your property listing is now verified
    'property_rejected',      -- Your property listing was rejected
    'token_payment_success',  -- Token payment confirmed
    'move_in_confirmed'       -- Move-in confirmed; payout triggered
);


-- =============================================================================
-- 3. HELPER FUNCTIONS & TRIGGERS
-- =============================================================================

-- Auto-update updated_at on any row modification
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;


-- =============================================================================
-- 4. TABLES
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 4.1 users
-- Core identity table. Role is immutable after creation.
-- phone_encrypted stores AES-256 encrypted phone number.
-- personality_embedding is a 1536-dim vector (OpenAI text-embedding-3-small).
-- -----------------------------------------------------------------------------
CREATE TABLE users (
    id                      UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    name                    TEXT            NOT NULL,
    email                   TEXT            UNIQUE NOT NULL,
    password_hash           TEXT            NOT NULL,           -- bcrypt, cost >= 12
    phone_encrypted         TEXT,                               -- AES-256 encrypted; NULL until user adds it
    role                    user_role       NOT NULL,
    personality_embedding   vector(1536),                       -- populated after lifestyle profile completion
    lifestyle_tags          TEXT[]          DEFAULT '{}',
    bio                     TEXT,
    budget_min              NUMERIC(12,2),                      -- tenants only
    budget_max              NUMERIC(12,2),                      -- tenants only
    preferred_localities    TEXT[]          DEFAULT '{}',       -- tenants only
    pending_embeddings      BOOLEAN         NOT NULL DEFAULT FALSE,
    is_active               BOOLEAN         NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    deleted_at              TIMESTAMPTZ
);

CREATE TRIGGER set_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();


-- -----------------------------------------------------------------------------
-- 4.2 landlord_kyc
-- One KYC record per landlord. Landlords cannot list until status = 'verified'.
-- Government IDs stored encrypted.
-- -----------------------------------------------------------------------------
CREATE TABLE landlord_kyc (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID        NOT NULL UNIQUE REFERENCES users(id),
    aadhaar_encrypted   TEXT,                           -- AES-256 encrypted
    pan_encrypted       TEXT,                           -- AES-256 encrypted
    aadhaar_verified    BOOLEAN     NOT NULL DEFAULT FALSE,
    pan_verified        BOOLEAN     NOT NULL DEFAULT FALSE,
    status              kyc_status  NOT NULL DEFAULT 'pending',
    submitted_at        TIMESTAMPTZ,
    verified_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

CREATE TRIGGER set_landlord_kyc_updated_at
    BEFORE UPDATE ON landlord_kyc
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();


-- -----------------------------------------------------------------------------
-- 4.3 properties
-- Represents a rentable unit (flat/room/studio) OR a PG building container.
-- For PG type: rent_amount is NULL; rent is set at the rooms level.
-- location uses GEOGRAPHY for accurate distance calculations in metres.
-- -----------------------------------------------------------------------------
CREATE TABLE properties (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        UUID            NOT NULL REFERENCES users(id),
    title           TEXT            NOT NULL,
    description     TEXT,
    property_type   property_type   NOT NULL,
    location        GEOGRAPHY(Point, 4326),              -- (longitude, latitude)
    address_text    TEXT,
    city            TEXT,
    locality        TEXT,
    rent_amount     NUMERIC(12,2),                       -- NULL for pg type
    deposit_amount  NUMERIC(12,2),
    total_capacity  INT,
    lifestyle_tags  TEXT[]          DEFAULT '{}',
    status          property_status NOT NULL DEFAULT 'draft',
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,

    -- PG properties must not have a rent at the property level
    CONSTRAINT pg_no_rent CHECK (
        property_type != 'pg' OR rent_amount IS NULL
    )
);

CREATE TRIGGER set_properties_updated_at
    BEFORE UPDATE ON properties
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();


-- -----------------------------------------------------------------------------
-- 4.4 rooms
-- Individual rentable rooms within a PG building.
-- property_id must reference a property with property_type = 'pg'.
-- Enforced at application layer (DB FK alone cannot check parent type).
-- -----------------------------------------------------------------------------
CREATE TABLE rooms (
    id                  UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id         UUID            NOT NULL REFERENCES properties(id),
    room_number         TEXT,                           -- e.g. "Room 4B", "Ground Floor East"
    room_type           room_type       NOT NULL,
    capacity            INT             NOT NULL CHECK (capacity > 0),
    current_occupancy   INT             NOT NULL DEFAULT 0 CHECK (current_occupancy >= 0),
    rent_amount         NUMERIC(12,2)   NOT NULL,
    deposit_amount      NUMERIC(12,2),
    status              property_status NOT NULL DEFAULT 'draft',
    created_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ,

    CONSTRAINT occupancy_within_capacity CHECK (current_occupancy <= capacity)
);

CREATE TRIGGER set_rooms_updated_at
    BEFORE UPDATE ON rooms
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();


-- -----------------------------------------------------------------------------
-- 4.5 property_images
-- Images for a property or a specific room within a PG.
-- image_hash stores perceptual hash (pHash) for AI stock image detection.
-- -----------------------------------------------------------------------------
CREATE TABLE property_images (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id      UUID        NOT NULL REFERENCES properties(id),
    room_id          UUID        REFERENCES rooms(id),  -- NULL if image is for the whole property
    storage_url      TEXT        NOT NULL,
    image_hash       TEXT,                              -- pHash; populated post-upload
    is_primary       BOOLEAN     NOT NULL DEFAULT FALSE,
    is_stock_flagged BOOLEAN     NOT NULL DEFAULT FALSE,
    uploaded_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


-- -----------------------------------------------------------------------------
-- 4.6 verifications
-- A property needs two approved verifications to display the Verified badge:
--   1. verification_type = 'ai_photo' AND status = 'approved'
--   2. verification_type IN ('manual', 'virtual_tour') AND status = 'approved'
-- Checked at the application layer.
-- -----------------------------------------------------------------------------
CREATE TABLE verifications (
    id                  UUID                PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id         UUID                NOT NULL REFERENCES properties(id),
    admin_id            UUID                REFERENCES users(id),   -- NULL until assigned
    verification_type   verification_type   NOT NULL,
    status              verification_status NOT NULL DEFAULT 'pending',
    notes               TEXT,
    verified_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

CREATE TRIGGER set_verifications_updated_at
    BEFORE UPDATE ON verifications
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();


-- -----------------------------------------------------------------------------
-- 4.7 squads
-- A group of 2-5 tenants renting together.
-- status = 'browsing' is the only state where property_id may be NULL (squad-first flow).
-- current_member_count is denormalized for quick checks; kept in sync by application.
-- -----------------------------------------------------------------------------
CREATE TABLE squads (
    id                      UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id             UUID            REFERENCES properties(id),   -- NULL when status = 'browsing'
    room_id                 UUID            REFERENCES rooms(id),        -- NULL unless targeting specific PG room
    name                    TEXT,
    status                  squad_status    NOT NULL DEFAULT 'browsing',
    payment_model           payment_model   NOT NULL DEFAULT 'leader_pays_all',
    max_size                INT             NOT NULL DEFAULT 5 CHECK (max_size >= 2 AND max_size <= 5),
    current_member_count    INT             NOT NULL DEFAULT 1 CHECK (current_member_count >= 1),
    created_by              UUID            NOT NULL REFERENCES users(id),
    total_deposit_collected NUMERIC(12,2)   NOT NULL DEFAULT 0,
    token_paid_at           TIMESTAMPTZ,
    move_in_confirmed_at    TIMESTAMPTZ,
    created_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    deleted_at              TIMESTAMPTZ,

    -- BR-08: property_id must be non-null for any status other than 'browsing'
    CONSTRAINT squad_property_required_unless_browsing CHECK (
        status = 'browsing' OR property_id IS NOT NULL
    ),

    -- member count cannot exceed squad capacity
    CONSTRAINT member_count_within_max CHECK (current_member_count <= max_size)
);

CREATE TRIGGER set_squads_updated_at
    BEFORE UPDATE ON squads
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();


-- -----------------------------------------------------------------------------
-- 4.8 squad_members
-- Join table tracking which users belong to which squad and their status/role.
-- A user can only have one active (non-left, non-rejected) membership per squad.
-- -----------------------------------------------------------------------------
CREATE TABLE squad_members (
    id          UUID                PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id    UUID                NOT NULL REFERENCES squads(id),
    user_id     UUID                NOT NULL REFERENCES users(id),
    role        squad_member_role   NOT NULL DEFAULT 'member',
    status      squad_member_status NOT NULL DEFAULT 'invited',
    share_amount NUMERIC(12,2),                  -- this member's contribution to the token
    joined_at   TIMESTAMPTZ,
    left_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,

    -- A user can only appear once per squad (at the DB level)
    CONSTRAINT unique_squad_member UNIQUE (squad_id, user_id)
);

CREATE TRIGGER set_squad_members_updated_at
    BEFORE UPDATE ON squad_members
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();


-- -----------------------------------------------------------------------------
-- 4.9 squad_lookups
-- Intent registry: a user declares they are looking for a squad.
-- property_id = NULL means squad-first flow (find people before property).
-- property_id = set means property-first flow (find squad-mates for a specific flat).
-- A user can only have one active lookup at a time (enforced via partial unique index).
-- Expires automatically after 30 days via application-level job.
-- -----------------------------------------------------------------------------
CREATE TABLE squad_lookups (
    id                   UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id              UUID            NOT NULL REFERENCES users(id),
    property_id          UUID            REFERENCES properties(id), -- NULL = squad-first
    locality_preference  TEXT,
    budget_min           NUMERIC(12,2),
    budget_max           NUMERIC(12,2),
    status               lookup_status   NOT NULL DEFAULT 'active',
    created_at           TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    expires_at           TIMESTAMPTZ     NOT NULL DEFAULT (NOW() + INTERVAL '30 days'),
    deleted_at           TIMESTAMPTZ
);


-- -----------------------------------------------------------------------------
-- 4.10 squad_property_proposals
-- Any squad member can propose a property for the squad to move into.
-- The squad leader is the sole decision-maker (Leader-Decides model).
-- On acceptance: squads.property_id is updated; all other pending proposals
-- for this squad are auto-rejected at the application layer.
-- -----------------------------------------------------------------------------
CREATE TABLE squad_property_proposals (
    id           UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id     UUID            NOT NULL REFERENCES squads(id),
    proposed_by  UUID            NOT NULL REFERENCES users(id),
    property_id  UUID            NOT NULL REFERENCES properties(id),
    room_id      UUID            REFERENCES rooms(id),  -- NULL unless targeting specific PG room
    status       proposal_status NOT NULL DEFAULT 'pending',
    proposed_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    resolved_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);


-- -----------------------------------------------------------------------------
-- 4.11 messages
-- Private chat messages within a squad.
-- read_by is a UUID array of user IDs who have read the message.
-- Pragmatic denormalization for squads of max 5 members.
-- NOTE: This table will be migrated to Redis Streams in Phase 2.
-- -----------------------------------------------------------------------------
CREATE TABLE messages (
    id           UUID                    PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id     UUID                    NOT NULL REFERENCES squads(id),
    sender_id    UUID                    NOT NULL REFERENCES users(id),
    content      TEXT                    NOT NULL,
    content_type message_content_type    NOT NULL DEFAULT 'text',
    sent_at      TIMESTAMPTZ             NOT NULL DEFAULT NOW(),
    read_by      UUID[]                  DEFAULT '{}',
    deleted_at   TIMESTAMPTZ
);


-- -----------------------------------------------------------------------------
-- 4.12 transactions
-- Financial ledger for all money movement on the platform.
-- A transaction record MUST be created before calling the payment gateway.
-- gateway_reference_id is the unique idempotency key from the gateway.
-- -----------------------------------------------------------------------------
CREATE TABLE transactions (
    id                      UUID                PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id                UUID                REFERENCES squads(id),   -- NULL for individual token payments
    user_id                 UUID                NOT NULL REFERENCES users(id),
    property_id             UUID                NOT NULL REFERENCES properties(id),
    type                    transaction_type    NOT NULL,
    amount                  NUMERIC(12,2)       NOT NULL CHECK (amount > 0),
    currency                TEXT                NOT NULL DEFAULT 'INR',
    gateway                 payment_gateway,
    gateway_reference_id    TEXT                UNIQUE,                  -- idempotency; set after gateway call
    gateway_status          TEXT,                                        -- raw status string from gateway
    status                  transaction_status  NOT NULL DEFAULT 'initiated',
    created_at              TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    settled_at              TIMESTAMPTZ,
    deleted_at              TIMESTAMPTZ
);


-- -----------------------------------------------------------------------------
-- 4.13 notifications
-- In-app notification feed. Each row is one notification for one user.
-- metadata (JSONB) carries context-specific payload, e.g.:
--   { "squad_id": "...", "property_id": "..." }
-- is_read / read_at track read state. Email delivery is handled separately
-- by the notification service and does NOT change this record's state.
-- -----------------------------------------------------------------------------
CREATE TABLE notifications (
    id           UUID               PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID               NOT NULL REFERENCES users(id),
    type         notification_type  NOT NULL,
    title        TEXT               NOT NULL,
    body         TEXT               NOT NULL,
    is_read      BOOLEAN            NOT NULL DEFAULT FALSE,
    read_at      TIMESTAMPTZ,
    metadata     JSONB,             -- e.g. {"squad_id": "...", "property_id": "..."}
    created_at   TIMESTAMPTZ        NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);


-- =============================================================================
-- 5. INDEXES
-- =============================================================================

-- Spatial: core map search feature
CREATE INDEX idx_properties_location
    ON properties USING GIST (location);

-- pgvector: Squad compatibility matching (cosine similarity)
-- lists = 100 is a starting point; tune based on dataset size
CREATE INDEX idx_users_embedding
    ON users USING ivfflat (personality_embedding vector_cosine_ops)
    WITH (lists = 100);

-- Soft-delete partial indexes (exclude deleted rows from index, biggest win on read queries)
CREATE INDEX idx_properties_status_active
    ON properties (status) WHERE deleted_at IS NULL;

CREATE INDEX idx_squads_status_active
    ON squads (status) WHERE deleted_at IS NULL;

CREATE INDEX idx_squad_lookups_active
    ON squad_lookups (property_id, status) WHERE deleted_at IS NULL;

CREATE INDEX idx_users_active
    ON users (email) WHERE deleted_at IS NULL;

-- FK indexes (prevent sequential scans on joins)
CREATE INDEX idx_rooms_property_id
    ON rooms (property_id);

CREATE INDEX idx_property_images_property_id
    ON property_images (property_id);

CREATE INDEX idx_property_images_room_id
    ON property_images (room_id);

CREATE INDEX idx_verifications_property_id
    ON verifications (property_id);

CREATE INDEX idx_squad_members_squad_id
    ON squad_members (squad_id);

CREATE INDEX idx_squad_members_user_id
    ON squad_members (user_id);

CREATE INDEX idx_squad_lookups_user_id
    ON squad_lookups (user_id);

CREATE INDEX idx_squad_proposals_squad_id
    ON squad_property_proposals (squad_id);

CREATE INDEX idx_squad_proposals_property_id
    ON squad_property_proposals (property_id);

-- Messages: always queried by squad, ordered by time (descending for latest-first)
CREATE INDEX idx_messages_squad_id_sent_at
    ON messages (squad_id, sent_at DESC);

CREATE INDEX idx_transactions_squad_id
    ON transactions (squad_id);

CREATE INDEX idx_transactions_user_id
    ON transactions (user_id);

CREATE INDEX idx_transactions_property_id
    ON transactions (property_id);

-- One active lookup per user (partial unique index)
CREATE UNIQUE INDEX idx_squad_lookups_one_active_per_user
    ON squad_lookups (user_id)
    WHERE status = 'active' AND deleted_at IS NULL;

-- Notifications: primary query pattern — fetch unread for a user, newest first
CREATE INDEX idx_notifications_user_unread
    ON notifications (user_id, created_at DESC)
    WHERE is_read = FALSE AND deleted_at IS NULL;

-- Notifications: FK index
CREATE INDEX idx_notifications_user_id
    ON notifications (user_id);

-- Notifications: full feed query (read + unread)
CREATE INDEX idx_notifications_user_created_at
    ON notifications (user_id, created_at DESC)
    WHERE deleted_at IS NULL;


-- =============================================================================
-- 6. SEED DATA
-- =============================================================================

-- Seed System Admin
INSERT INTO users (name, email, password_hash, role) 
VALUES ('System Admin', 'admin@example.com', crypt('Pass123!', gen_salt('bf', 12)), 'admin')
ON CONFLICT (email) DO NOTHING;
