# BachelorPad — Software Requirements Specification (SRS)
> **Version:** 1.1 (Expanded from Draft)
> **Date:** April 2026
> **Status:** Active — Backend & Database Phase

---

## Table of Contents
1. [Introduction](#1-introduction)
2. [General Description](#2-general-description)
3. [User Classes](#3-user-classes)
4. [System Features — Functional Requirements](#4-system-features)
5. [Database Schema](#5-database-schema)
6. [External Interfaces](#6-external-interfaces)
7. [Non-Functional Requirements](#7-non-functional-requirements)
8. [Frontend Architecture (Deferred — Phase 2)](#8-frontend-architecture-deferred)
9. [Business Rules Summary](#9-business-rules-summary)
10. [Open Items & Decisions Log](#10-open-items--decisions-log)

---

## 1. Introduction

### 1.1 Purpose
BachelorPad is a zero-brokerage rental marketplace designed specifically for bachelors (students and working professionals). The platform connects verified landlords to bachelor tenants through a trust-first approach, eliminating traditional broker middlemen and enabling social roommate matching via a "Squad" system.

### 1.2 Core Pillars
| Pillar | Description |
|--------|-------------|
| **Zero Tenant Brokerage** | Tenants pay zero platform fees. The platform earns a success fee from landlords only upon confirmed occupancy. |
| **Verified Listings** | AI photo verification + admin review gate every listing. No unverified properties reach tenants. |
| **Squad Matchmaking** | Bachelors can form groups (Squads) to pool resources and rent larger units together. Compatibility is calculated via vector embeddings (pgvector). |

### 1.3 Scope
The system allows:
- Tenants to search for verified rooms/flats/PGs on a map interface
- Tenants to form or join "Squads" to rent together
- Landlords to list properties (rooms, full flats, PGs with multiple rooms)
- Admins to verify listings and manage disputes
- The platform to hold Token payments in escrow until move-in is confirmed

### 1.4 Current Phase
**Phase 1:** Backend (Go microservices) + Database (PostgreSQL + PostGIS + pgvector) only.
**Phase 2 (Deferred):** Next.js web frontend, React Native mobile app, Redis chat layer.

### 1.5 Definitions & Acronyms
| Term | Definition |
|------|-----------|
| PG | Paying Guest accommodation — a property with multiple individually rentable rooms |
| Squad | A temporary group of 2–5 users formed on the platform to rent a single unit together |
| Room | A single rentable unit — can belong to a full flat (independent) or a PG building |
| Token Amount | A commitment fee paid by the Squad to "lock" a property, held in escrow |
| Lobby | The intermediate state where a Squad is forming but not yet committed to a property |
| GIS | Geographic Information System — used for map-based searches via PostGIS |
| KYC | Know Your Customer — identity verification for landlords via Aadhaar/PAN |

---

## 2. General Description

### 2.1 Architecture Overview
```
[Mobile App / Web Browser]
         |
         | HTTPS
         v
[Go REST API (Cloud Run)]
    |            |
    v            v
[PostgreSQL]  [External APIs]
 PostGIS       - Razorpay (payments)
 pgvector      - Google Maps (geocoding)
 (+ pgcrypto)  - OpenAI (embeddings)
               - AWS S3 (images, Phase 2)
```

### 2.2 Deployment Target
- **Backend:** Google Cloud Run (containerized Go service)
- **Database:** Google Cloud SQL (PostgreSQL 16+)
- **Frontend:** Vercel (Phase 2)

---

## 3. User Classes

### 3.1 Tenant (Bachelor)
- **Who:** Students and working professionals (18–35 age group) seeking affordable, trustworthy housing
- **Goals:** Find verified rooms quickly; avoid broker fees; find compatible roommates
- **Characteristics:** Mobile-first; cost-sensitive; values transparency and community
- **Permissions:** Browse listings, create/join Squads, pay Token Amount, submit move-in confirmation

### 3.2 Landlord / PG Owner
- **Who:** Property owners renting out rooms, full flats, or PG accommodations
- **Goals:** Find reliable bachelor tenants without a broker; reduce vacancy periods
- **Characteristics:** May manage multiple properties or a PG with 10–30 rooms
- **Permissions:** List properties/rooms after KYC; view tenant Squads interested in their property; receive payouts

### 3.3 Admin (Internal Staff)
- **Who:** BachelorPad operations team
- **Goals:** Verify listings; resolve disputes; manage platform health
- **Permissions:** Full read/write on all entities; can approve/reject verifications; can disburse or refund transactions

---

## 4. System Features

### 4.1 User Management

**FR-1.1:** Users can register via email + password. Phone number is optional at registration but required before booking.

**FR-1.2:** Role is selected at registration (`tenant` or `landlord`). Role cannot be changed post-registration.

**FR-1.3:** Users must complete a Lifestyle Profile during onboarding:
- Tags (multi-select): "Late-night friendly", "Veg-only", "WFH optimized", "Non-smoker", "Fitness-oriented", "Pet owner", etc.
- A short bio (text)
- Budget range and preferred localities
- This data is embedded into a 1536-dimension vector (via OpenAI API) and stored in `users.personality_embedding` for Squad matching.

**FR-1.4:** Landlords must complete KYC (Aadhaar + PAN verification) before any listing goes live.

---

### 4.2 Property & Room Listing

**FR-2.1:** Landlords can list two types of entities:
- **Full Flat / Room / Studio:** A single rentable unit. Has its own `properties` record with `property_type` set accordingly.
- **PG Building:** A parent `properties` record with `property_type = 'pg'`. Individual rooms are child records in the `rooms` table linked to this parent. Rent is set per room, not at the PG level.

**FR-2.2:** Each property/room listing requires:
- Title, description, address, GIS coordinates
- Rent amount, deposit amount, capacity
- At least 3 photos (uploaded to cloud storage)
- Lifestyle tag compatibility (which tenant lifestyle tags does this property support?)

**FR-2.3:** Newly submitted listings start with `status = 'pending_verification'`. They are invisible to tenants until verified.

**FR-2.4:** Landlords can manage room availability within a PG independently. A parent PG property is considered "fully occupied" only when all its child rooms are occupied.

---

### 4.3 Verified Listing Discovery

**FR-3.1 (Map Search):** Tenants search for properties using a map interface. The backend uses PostGIS `ST_DWithin` queries to return properties within a radius (default 2km, max 10km).

**FR-3.2 (Verified Badge):** The system displays a "Verified" badge only when:
- `verifications` has an `approved` record with `verification_type = 'ai_photo'`
- `verifications` has an `approved` record with `verification_type = 'manual'` or `'virtual_tour'`

**FR-3.3 (Lifestyle Filters):** Tenants can filter properties by lifestyle tags. Tags are stored as `TEXT[]` on both users and properties; the API uses the Postgres `&&` (overlap) operator for filtering.

**FR-3.4 (Price Integrity):** The listed rent must match the rent on the KYC/supporting document. Admins flag discrepancies. Clickbait pricing is a policy violation leading to delisting.

---

### 4.4 Squad Matchmaking

The Squad system supports two entry flows that coexist:

#### Flow A: Property-First
1. Tenant browses properties → finds one they like
2. Clicks **"Looking for Squad for this property"**
3. System creates a `squad_lookups` record with `property_id` set (NOT NULL)
4. Matching engine finds other users with `squad_lookups.property_id = same_property`
5. Compatibility score calculated via pgvector cosine similarity on `personality_embedding`
6. Users are invited to a Squad Lobby → Squad forms → Token paid → Property locked

#### Flow B: Squad-First
1. Tenant creates a "Looking for Roommates" profile (`squad_lookups` with `property_id = NULL`)
2. Provides: budget range, preferred locality, lifestyle tags
3. Matching engine finds compatible users with overlapping budget/locality and high embedding similarity
4. Users invited to a Squad Lobby → Squad forms with `status = 'browsing'`
5. Any Squad member proposes a property via `squad_property_proposals`
6. **Squad Leader accepts or rejects the proposal** (Leader-Decides model)
7. On acceptance → `squads.property_id` is updated → status advances to `forming`
8. Token paid → Property locked

**FR-4.1:** A user can have at most one active `squad_lookups` record at a time.

**FR-4.2:** Squad max size is 5. Enforced at DB (CHECK constraint) and application layer.

**FR-4.3 (Squad Status Flow):**
```
browsing -> forming -> locked -> moved_in -> disbanded
```
- `browsing`: Squad-first flow; no property selected yet
- `forming`: Property identified; members finalizing
- `locked`: Token paid; property reserved
- `moved_in`: Tenant confirmed move-in; landlord payout triggered
- `disbanded`: Group dissolved (refund may apply)

**FR-4.4 (Property Proposals):** Any Squad member can submit a `squad_property_proposals` record. The Squad leader is the sole decision-maker — they accept or reject. On acceptance, `squads.property_id` is set and all other open proposals for that Squad are automatically rejected.

**FR-4.5 (Private Chat):** Each Squad has a private chat room backed by the `messages` table. Messages are visible only to Squad members with `status = 'accepted'`.

---

### 4.5 AI Verification Pipeline

**FR-5.1:** When a landlord submits a listing, the system automatically:
1. Hashes all uploaded photos (perceptual hash / pHash)
2. Compares against stock image databases
3. Creates a `verifications` record with `verification_type = 'ai_photo'`
4. If no stock images detected → `status = 'approved'`; else → `status = 'rejected'`, landlord notified

**FR-5.2:** After AI approval, an admin manually reviews the listing. Creates a second `verifications` record with `verification_type = 'manual'` or `'virtual_tour'`.

**FR-5.3:** Both verifications must be `approved` for the Verified badge to display.

---

### 4.6 Transaction & Commitment

**FR-6.1 (Token Payment):** To transition a Squad from `forming` to `locked`:
1. A `transactions` record is created with `type = 'token_payment'` and `status = 'initiated'`
2. Squad Leader initiates payment via Razorpay
3. On webhook confirmation → `status = 'success'`, `squads.status = 'locked'`, property reserved

**FR-6.2 (Move-in Confirmation):**
1. Squad Leader confirms move-in
2. System creates a `transactions` record with `type = 'success_fee'`
3. Success fee charged to landlord; remaining balance transferred
4. `squads.status = 'moved_in'`, `properties.status = 'occupied'`

**FR-6.3 (Refund):** Admin-reviewed refund on Squad disbandment post-token. Recorded as `type = 'refund'`.

**FR-6.4 (Privacy Gate):** Landlord phone hidden until:
- The tenant's Squad has `status = 'locked'` for that property, OR
- The individual tenant has a `status = 'success'` `token_payment` transaction for that property

---

## 5. Database Schema

### 5.1 Extensions Required
```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS vector;
```

### 5.2 ENUM Types
```
user_role:               tenant | landlord | admin
property_type:           room | flat | pg | studio
property_status:         draft | pending_verification | verified | occupied | delisted
room_type:               single | double | triple | dormitory
squad_status:            browsing | forming | locked | moved_in | disbanded
squad_member_status:     invited | accepted | rejected | left
squad_member_role:       leader | member
proposal_status:         pending | accepted | rejected
verification_type:       ai_photo | manual | virtual_tour | physical
verification_status:     pending | approved | rejected
transaction_type:        token_payment | success_fee | refund | payout
transaction_status:      initiated | success | failed | refunded
kyc_status:              pending | verified | rejected
message_content_type:    text | system | file
lookup_status:           active | matched | inactive
payment_gateway:         razorpay | stripe
```

### 5.3 Tables

#### `users`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | gen_random_uuid() |
| name | TEXT NOT NULL | |
| email | TEXT UNIQUE NOT NULL | |
| password_hash | TEXT NOT NULL | bcrypt, cost >= 12 |
| phone_encrypted | TEXT | AES-256 encrypted |
| role | user_role NOT NULL | |
| personality_embedding | vector(1536) | OpenAI embedding |
| lifestyle_tags | TEXT[] | |
| bio | TEXT | |
| budget_min | NUMERIC(12,2) | Tenants only |
| budget_max | NUMERIC(12,2) | Tenants only |
| preferred_localities | TEXT[] | Tenants only |
| is_active | BOOLEAN DEFAULT TRUE | |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |
| deleted_at | TIMESTAMPTZ | Soft delete |

#### `landlord_kyc`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| user_id | UUID FK (users) UNIQUE | One KYC per landlord |
| aadhaar_encrypted | TEXT | |
| pan_encrypted | TEXT | |
| aadhaar_verified | BOOLEAN DEFAULT FALSE | |
| pan_verified | BOOLEAN DEFAULT FALSE | |
| status | kyc_status DEFAULT 'pending' | |
| submitted_at | TIMESTAMPTZ | |
| verified_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |
| deleted_at | TIMESTAMPTZ | |

#### `properties`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| owner_id | UUID FK (users) | Must be landlord role |
| title | TEXT NOT NULL | |
| description | TEXT | |
| property_type | property_type NOT NULL | |
| location | GEOGRAPHY(Point, 4326) | PostGIS |
| address_text | TEXT | |
| city | TEXT | |
| locality | TEXT | |
| rent_amount | NUMERIC(12,2) | NULL for PG type (rent at room level) |
| deposit_amount | NUMERIC(12,2) | |
| total_capacity | INT | |
| lifestyle_tags | TEXT[] | |
| status | property_status DEFAULT 'draft' | |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |
| deleted_at | TIMESTAMPTZ | |

#### `rooms` *(children of PG properties)*
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| property_id | UUID FK (properties) | Parent must be property_type = 'pg' |
| room_number | TEXT | e.g. "Room 4B" |
| room_type | room_type NOT NULL | |
| capacity | INT NOT NULL | |
| current_occupancy | INT DEFAULT 0 | |
| rent_amount | NUMERIC(12,2) NOT NULL | |
| deposit_amount | NUMERIC(12,2) | |
| status | property_status DEFAULT 'draft' | |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |
| deleted_at | TIMESTAMPTZ | |

#### `property_images`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| property_id | UUID FK (properties) | |
| room_id | UUID FK (rooms) NULLABLE | Set if image belongs to a specific room |
| storage_url | TEXT NOT NULL | |
| image_hash | TEXT | pHash for AI stock detection |
| is_primary | BOOLEAN DEFAULT FALSE | |
| is_stock_flagged | BOOLEAN DEFAULT FALSE | |
| uploaded_at | TIMESTAMPTZ | |

#### `verifications`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| property_id | UUID FK (properties) | |
| admin_id | UUID FK (users) NULLABLE | Set when human picks it up |
| verification_type | verification_type NOT NULL | |
| status | verification_status DEFAULT 'pending' | |
| notes | TEXT | |
| verified_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |
| deleted_at | TIMESTAMPTZ | |

#### `squads`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| property_id | UUID FK (properties) NULLABLE | NULL only when status = 'browsing' |
| room_id | UUID FK (rooms) NULLABLE | For squads targeting a specific PG room |
| name | TEXT | |
| status | squad_status DEFAULT 'browsing' | |
| max_size | INT DEFAULT 5 CHECK (max_size <= 5) | |
| current_member_count | INT DEFAULT 1 | Denormalized; keep in sync |
| created_by | UUID FK (users) | The initiating user |
| total_deposit_collected | NUMERIC(12,2) DEFAULT 0 | |
| token_paid_at | TIMESTAMPTZ | |
| move_in_confirmed_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |
| deleted_at | TIMESTAMPTZ | |

#### `squad_members`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| squad_id | UUID FK (squads) | |
| user_id | UUID FK (users) | |
| role | squad_member_role DEFAULT 'member' | |
| status | squad_member_status DEFAULT 'invited' | |
| share_amount | NUMERIC(12,2) | This member's token contribution |
| joined_at | TIMESTAMPTZ | |
| left_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |
| deleted_at | TIMESTAMPTZ | |

#### `squad_lookups` *(intent registry)*
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| user_id | UUID FK (users) | |
| property_id | UUID FK (properties) NULLABLE | NULL = squad-first flow |
| locality_preference | TEXT | |
| budget_min | NUMERIC(12,2) | |
| budget_max | NUMERIC(12,2) | |
| status | lookup_status DEFAULT 'active' | |
| created_at | TIMESTAMPTZ | |
| expires_at | TIMESTAMPTZ | Auto-expire after 30 days |
| deleted_at | TIMESTAMPTZ | |

#### `squad_property_proposals`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| squad_id | UUID FK (squads) | |
| proposed_by | UUID FK (users) | Any member can propose |
| property_id | UUID FK (properties) | |
| room_id | UUID FK (rooms) NULLABLE | |
| status | proposal_status DEFAULT 'pending' | Leader accepts/rejects |
| proposed_at | TIMESTAMPTZ | |
| resolved_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | |
| deleted_at | TIMESTAMPTZ | |

#### `messages`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| squad_id | UUID FK (squads) | |
| sender_id | UUID FK (users) | |
| content | TEXT NOT NULL | |
| content_type | message_content_type DEFAULT 'text' | |
| sent_at | TIMESTAMPTZ | |
| read_by | UUID[] | User IDs who have read this message |
| deleted_at | TIMESTAMPTZ | |

*Note: Chat will be migrated to Redis Streams in Phase 2.*

#### `transactions`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| squad_id | UUID FK (squads) NULLABLE | |
| user_id | UUID FK (users) | The payer |
| property_id | UUID FK (properties) | |
| type | transaction_type NOT NULL | |
| amount | NUMERIC(12,2) NOT NULL | |
| currency | TEXT DEFAULT 'INR' | |
| gateway | payment_gateway | |
| gateway_reference_id | TEXT UNIQUE | Idempotency key |
| gateway_status | TEXT | Raw gateway status |
| status | transaction_status DEFAULT 'initiated' | |
| created_at | TIMESTAMPTZ | |
| settled_at | TIMESTAMPTZ | |
| deleted_at | TIMESTAMPTZ | |

### 5.4 Key Indexes
```sql
-- Spatial search (core feature)
CREATE INDEX idx_properties_location ON properties USING GIST (location);

-- pgvector Squad matching
CREATE INDEX idx_users_embedding ON users USING ivfflat (personality_embedding vector_cosine_ops);

-- Soft delete partial indexes for high-traffic tables
CREATE INDEX idx_properties_active ON properties (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_squads_active ON squads (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_squad_lookups_active ON squad_lookups (property_id, status) WHERE deleted_at IS NULL;

-- FK indexes
CREATE INDEX idx_rooms_property_id ON rooms (property_id);
CREATE INDEX idx_squad_members_squad_id ON squad_members (squad_id);
CREATE INDEX idx_squad_members_user_id ON squad_members (user_id);
CREATE INDEX idx_messages_squad_id ON messages (squad_id, sent_at DESC);
CREATE INDEX idx_transactions_squad_id ON transactions (squad_id);
CREATE INDEX idx_verifications_property_id ON verifications (property_id);
CREATE INDEX idx_property_images_property_id ON property_images (property_id);
CREATE INDEX idx_squad_proposals_squad_id ON squad_property_proposals (squad_id);
```

---

## 6. External Interfaces

### 6.1 Payment — Razorpay
- Used for Token payments (escrow) and success fee collection
- Webhook-based status updates → backend verifies signature before processing
- `transactions` record created BEFORE calling gateway (idempotency)

### 6.2 Maps — Google Maps API
- Geocoding: Convert address text to lat/lng on property submission
- Reverse geocoding: lat/lng to human-readable address
- Street View: Embedded in property detail page (Phase 2 frontend)

### 6.3 Embeddings — OpenAI API
- Model: `text-embedding-3-small` (1536 dimensions)
- Called on: User Lifestyle Profile save, property tag updates
- Result stored in: `users.personality_embedding`

### 6.4 Image Storage — AWS S3 / Cloudinary
- Presigned URL upload flow (backend generates URL; client uploads directly)
- Image hash extracted after upload for AI verification
- **Phase 2 implementation**

---

## 7. Non-Functional Requirements

### 7.1 Performance
| Metric | Target |
|--------|--------|
| Map search query (PostGIS ST_DWithin) | < 500ms p95 |
| Squad matching query (pgvector cosine) | < 300ms p95 |
| API response time (non-geo) | < 200ms p95 |
| Concurrent WebSocket connections (Phase 2) | 5,000+ |

### 7.2 Security
| Area | Requirement |
|------|-------------|
| Data at rest | AES-256 for all PII (phone, Aadhaar, PAN) |
| Passwords | bcrypt cost >= 12 |
| Tokens | JWT HS256, 24h access + 7d refresh |
| Transport | TLS 1.3 minimum |
| Landlord identity | Aadhaar + PAN verified before listing goes live |

### 7.3 Availability
- **Target:** 99.9% uptime
- **Strategy:** Cloud Run auto-scaling + Cloud SQL High Availability

### 7.4 Data Integrity
- All financial transactions are idempotent (`gateway_reference_id` unique constraint)
- Squad member count is a denormalized field — kept in sync via application logic
- Property status is a forward-only state machine

---

## 8. Frontend Architecture (Deferred — Phase 2)

> [!NOTE]
> This section is documented for planning context only. No frontend work happens in Phase 1.

### 8.1 Web (Next.js 14 App Router)
- **Tenant Landing:** Map-based search, lifestyle filter panel, property cards
- **Property Detail:** Photos, verified badge, lifestyle tags, Squad CTA
- **Squad Lobby:** Member cards, compatibility scores, private chat, property proposals
- **Landlord Dashboard:** Property listing manager, room availability grid, KYC status, payout history
- **Admin Panel:** Verification queue, dispute resolution, user management

### 8.2 Mobile (React Native — Phase 3)
- High-performance map interactions
- Push notifications for Squad invites, chat messages, booking confirmations
- Camera integration for property photo uploads

---

## 9. Business Rules Summary

| Rule ID | Rule |
|---------|------|
| BR-01 | Tenants are never charged a platform fee |
| BR-02 | User role is set at registration and cannot change |
| BR-03 | Property status transitions are forward-only |
| BR-04 | Verified badge requires both AI photo approval AND manual/virtual admin approval |
| BR-05 | Squad max size is 5 members (hard limit) |
| BR-06 | Landlord phone hidden until Squad is `locked` OR individual token paid |
| BR-07 | Landlords can only list after `landlord_kyc.status = 'verified'` |
| BR-08 | `squads.property_id` is NULL only when `squads.status = 'browsing'` |
| BR-09 | Transaction record created BEFORE payment gateway call |
| BR-10 | Listed rent must match supporting document rent |
| BR-11 | PG rooms exist in `rooms` table; rent is set at room level, not PG property level |
| BR-12 | `squads.current_member_count` must equal count of accepted `squad_members` |
| BR-13 | Squad property selection uses Leader-Decides model via `squad_property_proposals` |

---

## 10. Open Items & Decisions Log

| # | Item | Decision | Date |
|---|------|----------|------|
| 1 | User roles | Mutually exclusive (tenant OR landlord) | April 2026 |
| 2 | PG room management | Separate `rooms` table with FK to parent `properties` | April 2026 |
| 3 | Squad entry flows | Both property-first AND squad-first supported | April 2026 |
| 4 | Squad property selection | Leader-Decides model via `squad_property_proposals` | April 2026 |
| 5 | Schema migrations | Manual SQL only | April 2026 |
| 6 | Soft deletes | Everywhere | April 2026 |
| 7 | Chat persistence | PostgreSQL (Phase 1); Redis Streams (Phase 2) | April 2026 |
| 8 | Payment gateway | Razorpay primary, Stripe secondary | April 2026 |
| 9 | Embedding model | OpenAI `text-embedding-3-small` (1536 dims) | April 2026 |
| 10 | ORM | None — raw pgx queries | April 2026 |

---

*End of SRS v1.1*
