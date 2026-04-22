# Backend Development Plan — Phase 1
> **Project:** NameNotDecided (product name: BachelorPad)
> **Go Module:** `namenotdecidedyet`
> **Date:** April 2026
> **Status:** 🟡 Planning Complete — Ready to Build

This document describes the Phase 1 backend build plan in full detail. Read `AI_AGENT_CONTEXT.md` first for conventions, business rules, and hard constraints.

---

## Strategy: Vertical Slice

Each module is built end-to-end (repository → service → handler → routes) before the next module starts. This means every module produces **working, curl-testable HTTP endpoints** when complete. No skeleton code sitting unconnected.

**Dependency order is respected:** Auth comes before everything else since all other endpoints require a JWT. KYC comes before Properties since listing requires verified KYC. Squad system comes after Properties since squads need verified properties.

---

## Module Overview

| # | Module | Status | Depends On |
|---|--------|--------|------------|
| 1 | Scaffold | ✅ Completed | — |
| 2 | Auth | ✅ Completed | 1 |
| 3 | User Profile | ✅ Completed | 2 |
| 4 | Landlord KYC | ✅ Completed | 2 |
| 5 | Properties & Rooms | ✅ Completed | 3, 4 |
| 6 | Verification Pipeline | ✅ Completed | 5 |
| 7 | Squad System | 🔲 In Progress | 3, 5 |
| 8 | Transactions & Payments | 🔲 Not started | 7 |
| 9 | Messages | 🔲 Not started | 7 |
| 10 | Notifications | 🔲 Not started | 2–9 |

---

## Module 1: Project Scaffold

**Goal:** `go run ./cmd/api/` starts an HTTP server. DB connects. Health endpoint returns 200.

### Files to Create

| File | Purpose |
|------|---------|
| `go.mod` | Module `namenotdecidedyet`, Go 1.22 |
| `go.sum` | Dependency lock (auto-generated) |
| `.env.example` | All required env vars with placeholder values |
| `.gitignore` | Standard Go gitignore |
| `Makefile` | `make run`, `make build`, `make lint` targets |
| `cmd/api/main.go` | Entry point — wires DB pool, config, router, starts server |
| `internal/config/config.go` | Typed `Config` struct; reads from env via `godotenv` |
| `internal/pkg/apierror/apierror.go` | Standard `APIError` type + error code constants |
| `internal/pkg/crypto/crypto.go` | AES-256 `Encrypt(plaintext)` / `Decrypt(ciphertext)` for PII |
| `internal/pkg/querybuilder/querybuilder.go` | Dynamic WHERE clause helper for pgx argument indexing |
| `internal/middleware/auth.go` | JWT validation; injects `user_id` + `role` into request context |
| `internal/middleware/cors.go` | CORS headers |
| `internal/middleware/logger.go` | Request logging (method, path, status, latency) |
| `internal/handler/router.go` | All routes registered here; chi router setup |

### Endpoint After Module
```
GET /api/v1/health
→ 200 { "success": true, "data": { "status": "ok" } }
```

### Key Design Notes
- `main.go` creates the `pgxpool.Pool`, passes it to all repos, starts chi router, and calls `http.ListenAndServe`.
- Config is loaded once at startup and passed as a struct — no global variables.
- `apierror.go` defines standard error codes: `VALIDATION_ERROR`, `NOT_FOUND`, `UNAUTHORIZED`, `FORBIDDEN`, `BUSINESS_RULE_VIOLATION`, `INTERNAL_ERROR`.
- All handlers return the standard envelope: `{ "success": bool, "data": any, "error": { "code": string, "message": string } }`.

---

## Module 2: Auth

**Goal:** Users can register and log in. A valid JWT is returned. Refresh token is stateless.

### Files to Create

| File | Purpose |
|------|---------|
| `internal/domain/user/user.go` | `User` struct, `RegisterInput`, `LoginInput`, domain errors |
| `internal/domain/user/service.go` | `Register()`, `Login()`, `RefreshToken()` business logic |
| `internal/repository/user_repo.go` | `CreateUser()`, `GetUserByEmail()`, `GetUserByID()` |
| `internal/handler/auth_handler.go` | HTTP handlers for register, login, refresh |

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/auth/register` | None | Create tenant or landlord account |
| `POST` | `/api/v1/auth/login` | None | Returns access token + refresh token |
| `POST` | `/api/v1/auth/refresh` | None (refresh token in body) | Issues new access token |

### Request / Response Examples

**Register:**
```json
POST /api/v1/auth/register
{
  "name": "Arjun Sharma",
  "email": "arjun@example.com",
  "password": "SecurePass123!",
  "role": "tenant"
}
→ 201 { "success": true, "data": { "user_id": "uuid", "role": "tenant" } }
```

**Login:**
```json
POST /api/v1/auth/login
{ "email": "arjun@example.com", "password": "SecurePass123!" }
→ 200 {
    "success": true,
    "data": {
      "access_token": "eyJ...",
      "refresh_token": "eyJ...",
      "expires_in": 86400
    }
  }
```

### Key Design Notes
- Password hashed with `bcrypt` cost 12 before storage.
- `role` at registration can only be `tenant` or `landlord`. `admin` is not a valid registration role (BR-02).
- Refresh token is a signed JWT with a longer expiry — no DB storage. Cannot be revoked before expiry.
- `deleted_at IS NULL` check always included in user lookups.

---

## Module 3: User Profile

**Goal:** Users fill out their Lifestyle Profile. The embedding is generated asynchronously.

### Files to Create

| File | Purpose |
|------|---------|
| `internal/domain/user/service.go` | Extended: `UpdateProfile()`, `GetProfile()` |
| `internal/repository/user_repo.go` | Extended: `UpdateProfile()`, `SetPendingEmbedding()` |
| `internal/handler/user_handler.go` | HTTP handlers for profile endpoints |
| `internal/pkg/embedding/worker.go` | Goroutine worker pool: polls `pending_embeddings`, calls OpenAI, stores vector |

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/users/me` | Required | Get own profile |
| `PUT` | `/api/v1/users/me/profile` | Required | Update lifestyle profile |

### Embedding Flow
```
PUT /api/v1/users/me/profile
  → saves: lifestyle_tags, bio, budget_min, budget_max, preferred_localities
  → sets: pending_embeddings = TRUE on users row
  → returns 200 immediately (embedding generated in background)

Background worker (goroutine pool):
  → SELECT id, lifestyle_tags, bio FROM users WHERE pending_embeddings = TRUE
  → calls OpenAI text-embedding-3-small API
  → UPDATE users SET personality_embedding = $1, pending_embeddings = FALSE WHERE id = $2
```

> [!NOTE]
> `pending_embeddings BOOLEAN` needs to be added to the `users` table. This will be a schema patch when Module 3 is implemented.

### Key Design Notes
- `personality_embedding` is never returned in any API response (too large, not useful to clients).
- If OpenAI call fails, the worker retries up to 3 times with exponential backoff before logging an error and moving on. The flag stays `TRUE` for the next poll cycle.

---

## Module 4: Landlord KYC

**Goal:** Landlord submits Aadhaar + PAN. Admin approves or rejects. Landlord cannot list until approved.

### Files to Create

| File | Purpose |
|------|---------|
| `internal/domain/kyc/kyc.go` | `LandlordKYC` struct, KYC domain errors |
| `internal/domain/kyc/service.go` | `SubmitKYC()`, `GetKYCStatus()`, `ApproveKYC()`, `RejectKYC()` |
| `internal/repository/kyc_repo.go` | DB access for `landlord_kyc` table |
| `internal/handler/kyc_handler.go` | HTTP handlers |

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/kyc` | Required (landlord) | Submit KYC documents |
| `GET` | `/api/v1/kyc/status` | Required (landlord) | Get own KYC status |
| `GET` | `/api/v1/admin/kyc` | Required (admin) | List all pending KYC submissions |
| `PUT` | `/api/v1/admin/kyc/{id}/approve` | Required (admin) | Approve KYC |
| `PUT` | `/api/v1/admin/kyc/{id}/reject` | Required (admin) | Reject KYC with notes |

### Key Design Notes
- `aadhaar_encrypted` and `pan_encrypted` are encrypted via `internal/pkg/crypto` before DB insert.
- Raw Aadhaar/PAN values never reach the DB or any log file.
- On approval/rejection, a notification is created for the landlord + email sent.
- BR-07: Any attempt to create a property before KYC is `verified` returns 403.

---

## Module 5: Properties & Rooms

**Goal:** Landlords list properties (flat/room/studio/PG). Tenants search on map using PostGIS.

### Files to Create

| File | Purpose |
|------|---------|
| `internal/domain/property/property.go` | `Property`, `Room`, `PropertyImage` structs, state machine |
| `internal/domain/property/service.go` | `CreateProperty()`, `SearchProperties()`, `AddRoom()`, etc. |
| `internal/repository/property_repo.go` | All property DB queries including PostGIS search |
| `internal/repository/room_repo.go` | Room-specific DB queries |
| `internal/handler/property_handler.go` | HTTP handlers |

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/properties` | Required (landlord, KYC verified) | Create a property listing |
| `GET` | `/api/v1/properties` | Optional | Map search with filters |
| `GET` | `/api/v1/properties/{id}` | Optional | Get property details |
| `PUT` | `/api/v1/properties/{id}` | Required (owner) | Update listing |
| `POST` | `/api/v1/properties/{id}/images` | Required (owner) | Add image URLs |
| `GET` | `/api/v1/properties/{id}/images` | Optional | List images |
| `POST` | `/api/v1/properties/{id}/rooms` | Required (owner, PG only) | Add room to PG |
| `GET` | `/api/v1/properties/{id}/rooms` | Optional | List PG rooms |
| `PUT` | `/api/v1/properties/{id}/rooms/{roomId}` | Required (owner) | Update room |

### Map Search Query Parameters

| Param | Type | Required | Notes |
|-------|------|----------|-------|
| `lat` | float | Yes | Latitude of search center |
| `lng` | float | Yes | Longitude of search center |
| `radius` | int | No | Metres, default 2000, max 10000 |
| `property_type` | string | No | `room`, `flat`, `pg`, `studio` |
| `price_min` | float | No | |
| `price_max` | float | No | |
| `lifestyle_tags` | string[] | No | Comma-separated; `&&` overlap filter |
| `page` | int | No | Default 1 |
| `per_page` | int | No | Default 20 |

### Core PostGIS Query
```sql
SELECT
    p.id, p.title, p.property_type, p.rent_amount, p.lifestyle_tags,
    ST_AsGeoJSON(p.location) AS location_geojson,
    ST_Distance(p.location, ST_MakePoint($1, $2)::geography) AS distance_metres
FROM properties p
WHERE
    p.deleted_at IS NULL
    AND p.status = 'verified'
    AND ST_DWithin(p.location, ST_MakePoint($1, $2)::geography, $3)
    -- optional filters appended by querybuilder helper
ORDER BY distance_metres ASC
LIMIT $? OFFSET $?
```

### Key Design Notes
- On property creation, call Google Maps Geocoding API to convert `address_text` to lat/lng. Store in `location GEOGRAPHY(Point, 4326)`.
- PG properties: `rent_amount` must be NULL at creation (BR-11). The service enforces this.
- Property status starts as `draft`. It moves to `pending_verification` when the landlord submits it for review.
- Property status transitions are **forward-only** (BR-03). The service validates state machine.

---

## Module 6: Verification Pipeline

**Goal:** Properties enter an admin queue. Admin manually verifies. Verified badge requires both ai_photo AND manual approval.

### Files to Create

| File | Purpose |
|------|---------|
| `internal/domain/verification/verification.go` | `Verification` struct, badge eligibility logic |
| `internal/domain/verification/service.go` | `CreateAIVerification()`, `ApproveVerification()`, `RejectVerification()`, `GetBadgeStatus()` |
| `internal/repository/verification_repo.go` | DB access for `verifications` table |
| `internal/handler/verification_handler.go` | HTTP handlers |

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/admin/verifications` | Admin | Paginated queue of pending verifications |
| `POST` | `/api/v1/admin/verifications/{propertyId}/ai` | Admin | Trigger Phase 1 stub AI check (creates pending ai_photo record) |
| `PUT` | `/api/v1/admin/verifications/{id}/approve` | Admin | Approve a verification |
| `PUT` | `/api/v1/admin/verifications/{id}/reject` | Admin | Reject with notes |
| `GET` | `/api/v1/properties/{id}/verification-status` | Optional | Returns badge eligibility |

### Badge Eligibility Logic
```
A property has a Verified badge IF:
  verifications WHERE property_id = X AND status = 'approved' AND type = 'ai_photo'   → exists
  AND
  verifications WHERE property_id = X AND status = 'approved' AND type IN ('manual', 'virtual_tour', 'physical') → exists
```
This is computed at the application layer (service), not as a DB view.

---

## Module 7: Squad System

**Goal:** Full squad lifecycle — intent registration, compatibility matching, squad creation, invites, property proposals.

### Files to Create

| File | Purpose |
|------|---------|
| `internal/domain/squad/squad.go` | `Squad`, `SquadMember`, `SquadLookup`, `SquadProposal` structs |
| `internal/domain/squad/service.go` | All squad business logic |
| `internal/repository/squad_repo.go` | Squad, member, lookup, proposal DB queries |
| `internal/handler/squad_handler.go` | HTTP handlers |

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/squad-lookups` | Required (tenant) | Register intent (property-first or squad-first) |
| `DELETE` | `/api/v1/squad-lookups` | Required (tenant) | Cancel active lookup |
| `GET` | `/api/v1/squad-lookups/matches` | Required (tenant) | Paginated compatible users |
| `POST` | `/api/v1/squads` | Required (tenant) | Create a squad |
| `GET` | `/api/v1/squads/{id}` | Required (member) | Get squad details |
| `POST` | `/api/v1/squads/{id}/invite` | Required (member) | Invite a matched user |
| `PUT` | `/api/v1/squads/{id}/members/me` | Required (invited user) | Accept or reject invitation |
| `DELETE` | `/api/v1/squads/{id}/members/me` | Required (member) | Leave a squad |
| `POST` | `/api/v1/squads/{id}/proposals` | Required (member) | Propose a property |
| `GET` | `/api/v1/squads/{id}/proposals` | Required (member) | List all proposals for squad |
| `PUT` | `/api/v1/squads/{id}/proposals/{proposalId}` | Required (leader) | Accept or reject proposal |
| `DELETE` | `/api/v1/squads/{id}` | Required (leader) | Disband squad |

### Squad Matching Query (pgvector)
```sql
SELECT
    u.id, u.name, u.lifestyle_tags, u.bio, u.budget_min, u.budget_max,
    1 - (u.personality_embedding <=> $1) AS compatibility_score
FROM users u
JOIN squad_lookups sl ON sl.user_id = u.id
WHERE
    u.deleted_at IS NULL
    AND u.id != $2                          -- exclude self
    AND sl.status = 'active'
    AND sl.deleted_at IS NULL
    AND sl.expires_at > NOW()
    AND 1 - (u.personality_embedding <=> $1) >= 0.7   -- threshold: BR decision
    -- optional: AND sl.property_id = $3 for property-first flow
ORDER BY compatibility_score DESC
LIMIT 10 OFFSET $?
```

### Key Business Rules Enforced
- **BR-05:** Squad max 5 members. Service rejects invites when `current_member_count >= max_size`.
- **BR-08:** `squads.property_id` must be non-null once status advances past `browsing`. The service enforces this on proposal acceptance.
- **BR-12:** `current_member_count` is updated atomically whenever a member accepts or leaves.
- **BR-13:** Only the squad leader can accept/reject proposals. Service checks `squad_members.role = 'leader'` for the requesting user.
- **FR-4.4:** On proposal acceptance, all other `pending` proposals for the same squad are auto-rejected in the same transaction.

---

## Module 8: Transactions & Payments

**Goal:** Token payment flow with full Razorpay integration. Move-in confirmation triggers success fee.

> [!IMPORTANT]
> This module is built **after** Modules 1–7 are complete and stable. Until then, a stub confirm endpoint is used for development testing.

### Files to Create

| File | Purpose |
|------|---------|
| `internal/domain/transaction/transaction.go` | `Transaction` struct, payment state machine |
| `internal/domain/transaction/service.go` | `InitiateTokenPayment()`, `HandleWebhook()`, `ConfirmMoveIn()` |
| `internal/repository/transaction_repo.go` | DB access for `transactions` table |
| `internal/handler/transaction_handler.go` | HTTP handlers including webhook |

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/transactions` | Required (squad leader) | Create transaction + Razorpay order |
| `POST` | `/api/v1/transactions/{id}/confirm` | Required (dev stub only) | Simulate webhook for testing |
| `POST` | `/api/v1/transactions/webhook` | None (Razorpay calls this) | Process Razorpay webhook |
| `POST` | `/api/v1/squads/{id}/move-in` | Required (squad leader) | Confirm move-in; trigger success fee |
| `GET` | `/api/v1/transactions` | Required | Get own transaction history |

### Payment Flow
```
1. POST /api/v1/transactions
   → creates transactions row (status = 'initiated')
   → calls Razorpay Orders API → gets order_id
   → returns { razorpay_order_id, amount, currency } to client

2. Client completes payment on Razorpay UI

3. POST /api/v1/transactions/webhook  (called by Razorpay)
   → verify X-Razorpay-Signature header (HMAC-SHA256)
   → update transactions.status = 'success'
   → update squads.status = 'locked'
   → update squads.token_paid_at = NOW()
   → create in-app notification + send email to squad members
```

### Key Design Notes
- **BR-09:** Transaction record ALWAYS created before gateway call. If gateway call fails, record stays `initiated`.
- `gateway_reference_id` has a UNIQUE constraint — this is the idempotency key. Duplicate webhooks are safely ignored.
- Webhook handler must return 200 to Razorpay even if processing fails internally (to prevent infinite retries). Log the failure.

---

## Module 9: Messages

**Goal:** Squad private chat stored in PostgreSQL. Paginated history. Read receipts via `read_by` array.

### Files to Create

| File | Purpose |
|------|---------|
| `internal/repository/message_repo.go` | `CreateMessage()`, `GetMessages()`, `MarkRead()` |
| `internal/handler/message_handler.go` | HTTP handlers |

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/squads/{id}/messages` | Required (accepted member) | Paginated message history (cursor-based) |
| `POST` | `/api/v1/squads/{id}/messages` | Required (accepted member) | Send a message |
| `PUT` | `/api/v1/squads/{id}/messages/read` | Required (accepted member) | Mark messages as read (updates `read_by`) |

### Key Design Notes
- Pagination is **cursor-based** (using `sent_at` timestamp as cursor) not offset-based — prevents duplicate/missing messages as new ones arrive.
- Only users with `squad_members.status = 'accepted'` for that squad can read or send messages (FR-4.5).
- `read_by UUID[]` array updated via `array_append` on read — safe for squads ≤ 5 members.

---

## Module 10: Notifications

**Goal:** In-app notification feed. Email sent on critical events (KYC, payment, verification).

### Files to Create

| File | Purpose |
|------|---------|
| `internal/repository/notification_repo.go` | `CreateNotification()`, `GetNotifications()`, `MarkRead()` |
| `internal/handler/notification_handler.go` | HTTP handlers |
| `internal/pkg/email/email.go` | SMTP wrapper: `SendEmail(to, subject, body)` |

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/notifications` | Required | Paginated notification feed (unread first) |
| `PUT` | `/api/v1/notifications/{id}/read` | Required | Mark one as read |
| `PUT` | `/api/v1/notifications/read-all` | Required | Mark all as read |

### Notification Triggers (Internal — No Direct Endpoint)

| Event | Notification Type | Email? |
|-------|-------------------|--------|
| KYC approved | `kyc_approved` | ✅ |
| KYC rejected | `kyc_rejected` | ✅ |
| Property verified | `property_verified` | ✅ |
| Property rejected | `property_rejected` | ✅ |
| Squad invite received | `squad_invite` | ❌ |
| Squad invite accepted | `squad_invite_accepted` | ❌ |
| Squad invite rejected | `squad_invite_rejected` | ❌ |
| Squad disbanded | `squad_disbanded` | ✅ |
| Property proposal submitted | `property_proposal` | ❌ |
| Proposal accepted | `proposal_accepted` | ❌ |
| Proposal rejected | `proposal_rejected` | ❌ |
| Token payment success | `token_payment_success` | ✅ |
| Move-in confirmed | `move_in_confirmed` | ✅ |

### Key Design Notes
- Email is **best-effort**. Never block the main operation if email fails. Log the failure.
- The `metadata JSONB` field carries contextual data for frontend deep-linking, e.g. `{"squad_id": "...", "property_id": "..."}`.

---

## Environment Variables

```env
# Server
PORT=8080
ENV=development                         # development | production

# Database
DATABASE_URL=postgres://user:pass@localhost:5432/bachelorpaddb?sslmode=disable

# JWT
JWT_SECRET=your-256-bit-secret-here    # Used for both access and refresh tokens

# AES-256 Encryption (PII fields: phone, Aadhaar, PAN)
# Must be exactly 32 bytes (hex-encode a random 32-byte value)
ENCRYPTION_KEY=0000000000000000000000000000000000000000000000000000000000000000

# Google Maps (for geocoding on property creation)
GOOGLE_MAPS_API_KEY=

# OpenAI (for personality embeddings — async worker)
OPENAI_API_KEY=

# Razorpay (wired in Module 8)
RAZORPAY_KEY_ID=
RAZORPAY_KEY_SECRET=

# Email / SMTP (wired in Module 10)
SMTP_HOST=smtp.sendgrid.net
SMTP_PORT=587
SMTP_USER=apikey
SMTP_PASS=
EMAIL_FROM=noreply@bachelorpad.in
```

---

## Schema Patches Needed Before/During Build

| Patch | When Needed | Description |
|-------|-------------|-------------|
| `pending_embeddings` column on `users` | Module 3 | Boolean flag for async embedding queue |

All patches go in `db/patches/YYYY-MM-DD_description.sql`. `schema_initializer.sql` is updated to reflect the canonical state after each patch.

---

*End of Backend Development Plan v1.0*
