# BachelorPad — AI Agent Context & Constraints
> **Version:** 1.0 | **Last Updated:** April 2026
> This file is the single source of truth for any AI agent, developer, or tool working on this codebase.
> Read this file completely before making ANY changes.

---

## 1. What This Project Is

**BachelorPad** is a zero-brokerage rental marketplace for bachelors (students and working professionals).
It is NOT a general-purpose rental platform. Every decision must align with these three core pillars:

| Pillar | Meaning |
|--------|---------|
| **Zero Tenant Brokerage** | Tenants never pay a platform fee. Revenue comes only from landlords (success fee on occupancy). |
| **Trust & Verification** | Every listing must pass AI photo verification + admin review before showing a "Verified" badge. |
| **Squad Matchmaking** | The platform's social differentiator — matching compatible bachelors into roommate groups. |

---

## 2. Current Development Scope

> [!IMPORTANT]
> **Backend and Database ONLY.** Frontend (Next.js) is explicitly out of scope for the current phase.
> Do NOT create, scaffold, or modify any frontend code. Do NOT install frontend dependencies.
> Frontend architecture is documented for context only.

### In Scope (Phase 1)
- PostgreSQL database schema design and management
- Go microservice backend (REST API)
- PostGIS for geospatial queries
- pgvector for personality/compatibility matching
- API endpoint design and implementation
- Business logic and domain rules
- Database query optimization

### Deferred (Phase 2+)
- Next.js frontend (web)
- React Native / mobile app
- WebSocket real-time chat (infrastructure decision deferred)
- Redis migration for chat (currently Postgres, migrate later)
- Payment gateway integration (Razorpay/Stripe — design now, implement later)
- AWS S3/Cloudinary for image storage (design now, implement later)

---

## 3. Technology Stack

### Backend
| Layer | Technology | Version/Notes |
|-------|-----------|---------------|
| **Go Module Name** | `namenotdecidedyet` | Private module name — no VCS prefix. All imports start with `namenotdecidedyet/internal/...` |
| Language | **Go** | 1.22+ |
| Web Framework | **chi** | Lightweight, idiomatic Go router with middleware support |
| Database Driver | **pgx/v5** | Native Postgres driver, NOT GORM |
| Migrations | **Manual SQL** | No migration framework. SQL files only. |
| Config | **godotenv** + structured config struct | |
| Logging | **zerolog** or **slog** (stdlib) | Structured JSON logging only |
| Validation | **go-playground/validator** | |
| Auth | **JWT (golang-jwt/jwt/v5)** | HS256, stateless. Access token 24h, refresh token 7d. |

### Database
| Component | Technology | Notes |
|-----------|-----------|-------|
| Primary DB | **PostgreSQL 16+** | Hosted in Docker during development |
| Geospatial | **PostGIS 3.4+** | Required for `GEOGRAPHY` types and distance queries |
| Vector Search | **pgvector 0.7+** | Required for personality embedding similarity |
| Extensions | `uuid-ossp`, `postgis`, `vector` | Enabled in `schema_initializer.sql` |

### Infrastructure (Development)
- **Docker Desktop** for running PostgreSQL + PostGIS + pgvector.
- **Go** running natively on host (Windows).

### Infrastructure (Target)
- **Google Cloud Run** for Go microservices.
- **Google Cloud SQL** for managed PostgreSQL.

---

## 4. Mandatory File & Folder Structure

> [!IMPORTANT]
> All new code MUST follow this structure. Do not create files outside this hierarchy without explicit instruction.

```
/BachelorPad (project root)
|
+-- cmd/
|   +-- api/
|       +-- main.go                  <- Single entry point. Wires everything together.
|
+-- internal/                        <- All private application code
|   +-- config/
|   |   +-- config.go                <- Reads env vars, returns typed Config struct
|   |
|   +-- domain/                      <- Pure business logic. NO database imports here.
|   |   +-- user/
|   |   |   +-- user.go              <- Structs, business rules, domain errors
|   |   |   +-- service.go           <- Orchestrates user-related operations
|   |   +-- property/
|   |   +-- squad/
|   |   +-- verification/
|   |   +-- transaction/
|   |
|   +-- repository/                  <- Database access layer (pgx queries)
|   |   +-- user_repo.go
|   |   +-- property_repo.go
|   |   +-- squad_repo.go
|   |   +-- ...
|   |
|   +-- handler/                     <- HTTP handlers. Thin layer — no business logic.
|   |   +-- user_handler.go
|   |   +-- property_handler.go
|   |   +-- squad_handler.go
|   |   +-- router.go                <- All routes registered here
|   |
|   +-- middleware/
|   |   +-- auth.go                  <- JWT validation middleware
|   |   +-- logger.go
|   |   +-- cors.go
|   |
|   +-- pkg/                         <- Shared utilities (no domain logic)
|       +-- apierror/
|       |   +-- apierror.go          <- Standard API error types
|       +-- validator/
|       +-- crypto/                  <- AES-256 encrypt/decrypt for PII (phone, Aadhaar, PAN)
|       +-- querybuilder/            <- In-house dynamic WHERE helper for pgx (NOT an ORM)
|       |   +-- querybuilder.go      <- Manages $1,$2... arg indexing for optional filter queries
|       +-- embedding/
|           +-- worker.go            <- Goroutine pool that calls OpenAI to generate user embeddings async
|
+-- db/
|   +-- schema_initializer.sql       <- THE canonical schema. Mounted to Docker init.
|
+-- docs/
|   +-- AI_AGENT_CONTEXT.md          <- This file
|   +-- PROJECT_SRS.md               <- Full Software Requirements Specification
|   +-- BACKEND_DEVELOPMENT_PLAN.md  <- Phase 1 build order, module breakdown, endpoints
|
+-- .env.example                     <- Template for environment variables
+-- .gitignore
+-- go.mod
+-- go.sum
+-- Makefile                         <- Common dev commands
```

---

## 5. Database Design Rules

### 5.1 Absolute Rules
- **All primary keys are `UUID`** — generated using `gen_random_uuid()` (built into PG 14+).
- **All tables have `created_at TIMESTAMPTZ` and `updated_at TIMESTAMPTZ`** — set via trigger.
- **All tables have `deleted_at TIMESTAMPTZ`** — soft delete everywhere. Never `DELETE` a row in application code.
- **All monetary values use `NUMERIC(12,2)`** — never `FLOAT` or `DOUBLE PRECISION`.
- **All timestamps are `TIMESTAMPTZ`** (timezone-aware) — never `TIMESTAMP`.
- **All phone numbers and government IDs are stored encrypted** (`_encrypted` suffix on column name).
- **PostGIS coordinates use `SRID 4326`** (WGS 84 — standard GPS coordinates).

### 5.2 ENUM Types
All ENUMs are defined as Postgres `TYPE` in the schema initializer, not as `VARCHAR` with check constraints.

### 5.3 Schema Change Process
> [!WARNING]
> There is NO automated migration framework. Schema changes are handled as follows:
> 1. AI agent writes the SQL patch.
> 2. The SQL patch is saved in `db/patches/YYYY-MM-DD_description.sql`.
> 3. The human developer reviews and executes it manually.
> 4. `schema_initializer.sql` is updated to reflect the new canonical state.
> **Never modify `schema_initializer.sql` without also creating the corresponding patch.**

### 5.4 Indexing Conventions
- Always index foreign key columns.
- Always add a `GIST` index on `GEOGRAPHY` columns used in spatial queries.
- Always add an `ivfflat` index on `vector` columns used with pgvector.
- Partial indexes on `deleted_at IS NULL` for all high-traffic queries.

---

## 6. Business Rules (Non-Negotiable)

These are invariants. They must NEVER be violated by any code change:

| Rule ID | Rule |
|---------|------|
| BR-01 | A tenant user (`role = 'tenant'`) can never be charged a platform fee. |
| BR-02 | A user cannot simultaneously be a `tenant` and a `landlord`. Roles are mutually exclusive. |
| BR-03 | A property's `status` cannot transition backwards (e.g., `verified -> pending_verification`). Only forward. |
| BR-04 | A property can only display a "Verified" badge if `verifications` has an `approved` record of type `ai_photo` AND `manual`. |
| BR-05 | A Squad can have a maximum of 5 members. Hard limit enforced at DB and application layer. |
| BR-06 | A tenant's phone number is NEVER exposed in any API response unless the requesting user is in the same Squad OR has paid the Token Amount for the same property. |
| BR-07 | A landlord can only list properties after `landlord_kyc.verification_status = 'verified'`. |
| BR-08 | `squads.property_id` can be NULL only when `squads.status = 'browsing'`. All other statuses require a non-null property. |
| BR-09 | A `transaction` record is created BEFORE calling the payment gateway. Status starts as `initiated`. |
| BR-10 | Rent amounts on the platform must match the submitted supporting document. No inflation allowed. |
| BR-11 | A property with `property_type = 'pg'` is a parent container only. Rent is set at the `rooms` level, not the property level. |
| BR-12 | `squads.current_member_count` must always equal the count of `squad_members` rows with `status = 'accepted'` for that squad. |
| BR-13 | Squad property selection uses the Leader-Decides model: any member proposes via `squad_property_proposals`, the Squad leader accepts or rejects. |

---

## 7. API Design Conventions

### 7.1 URL Structure
```
/api/v1/{resource}
/api/v1/{resource}/{id}
/api/v1/{resource}/{id}/{sub-resource}
```
Examples:
- `GET /api/v1/properties?lat=...&lng=...&radius=...`
- `POST /api/v1/squads`
- `GET /api/v1/squads/{id}/messages`
- `POST /api/v1/squads/{id}/proposals`

### 7.2 Response Envelope
All API responses must use this structure:
```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 150
  }
}
```
On error:
```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "PROPERTY_NOT_FOUND",
    "message": "The requested property does not exist or has been removed."
  }
}
```

### 7.3 HTTP Status Code Conventions
| Scenario | Code |
|----------|------|
| Success (read) | 200 |
| Success (created) | 201 |
| Bad request / validation error | 400 |
| Unauthorized (no/invalid token) | 401 |
| Forbidden (valid token, wrong permission) | 403 |
| Not found | 404 |
| Business rule violation | 422 |
| Server error | 500 |

---

## 8. Security Rules

- **PII Encryption:** `phone`, `aadhaar_number`, `pan_number` must be encrypted at rest using AES-256 via the `/internal/pkg/crypto` package. Raw values never go to the DB.
- **No Raw SQL from User Input:** All queries must use parameterized statements via `pgx`. String concatenation in SQL is forbidden.
- **JWT Claims:** Tokens must contain `user_id`, `role`, and `exp`. No sensitive data in JWT payload.
- **Admin Endpoints:** Any route under `/api/v1/admin/` must validate `role = 'admin'` at the middleware layer.

---

## 9. What to Never Do

> [!CAUTION]
> Violating any of these may compromise data integrity, security, or architecture.

- Do NOT use an ORM (GORM, Ent, etc.). Raw `pgx` queries only.
- Do NOT add external query builder libraries (squirrel, etc.). Use the in-house `internal/pkg/querybuilder` helper for dynamic filters.
- Do NOT store money as `FLOAT`. Always `NUMERIC(12,2)`.
- Do NOT write business logic in handlers. Handlers call services. Services call repositories.
- Do NOT expose raw database errors to API clients. Map to `apierror` types.
- Do NOT hard-delete rows. Set `deleted_at` only.
- Do NOT create frontend files, components, or pages in Phase 1.
- Do NOT run migration tools automatically. Write SQL, save as patch, inform the developer.
- Do NOT store unencrypted phone numbers or government IDs.
- Do NOT skip the `deleted_at IS NULL` filter in queries — this is a soft-delete codebase.
- Do NOT use `SELECT *` in repository queries. Always name columns explicitly.
- Do NOT call OpenAI in the HTTP request cycle. Embedding generation is always async via the goroutine worker pool.
- Do NOT expose landlord phone numbers in any API response unless the requesting user's Squad has `status = 'locked'` for that property (BR-06).

---

## 10. Dependency Decisions Log

| Decision | Choice | Reason |
|----------|--------|--------|
| Go module name | `namenotdecidedyet` | Private module; name TBD; all imports use this prefix |
| ORM | None (raw pgx) | Full control over queries; PostGIS and pgvector queries are complex |
| Dynamic query building | In-house `querybuilder` pkg | Avoids external deps; only needed for the map search endpoint |
| Router | chi | Idiomatic Go, middleware support, no magic |
| Refresh token storage | Stateless | Simpler; no DB table needed; cannot revoke before expiry |
| Admin login | Same `/api/v1/auth/login` endpoint | Role in JWT (`role = 'admin'`) differentiates access |
| Admin creation | Manual SQL seed | No self-registration for admins; operations team manages this |
| Image upload (Phase 1) | Accept `storage_url` string only | No cloud storage configured yet; real upload flow in Phase 2 |
| AI photo verification (Phase 1) | Stub — creates `pending` ai_photo verification | pHash requires image bytes; we only have URLs in Phase 1 |
| Embedding generation | Async goroutine worker pool | Never block the HTTP request; `pending_embeddings` flag on user |
| Embedding model | OpenAI `text-embedding-3-small` (1536 dims) | Best cost/quality ratio for personality matching |
| Squad matching threshold | Cosine similarity ≥ 0.7 | Below 0.7 = excluded from results |
| Squad match pagination | 10 per page | Keeps response fast; user scrolls to see more |
| Squad match selection | User manually picks | No auto-invite; user reviews list and chooses |
| Notification delivery | In-app DB table + Email (SMTP) | In-app for feed; email for critical events (KYC, payment) |
| Chat persistence | PostgreSQL (for now) | Simple. Redis Streams migration planned in Phase 2. |
| Migration tool | None (manual SQL) | Developer preference; all patches reviewed before execution |
| Payment gateway | Razorpay (primary), Stripe (secondary) | India-first market; wired after all basic features complete |
| Image CDN | AWS S3 / Cloudinary | Decision deferred to Phase 2 |
| Frontend | Next.js | Phase 2 — not in current scope |
| Squad property selection | Leader-Decides model | Simple, clear authority; avoids voting complexity in Phase 1 |

---

## 11. Phase 1 Locked Architecture Decisions

> [!IMPORTANT]
> These decisions were finalized during the backend planning session (April 2026).
> Do NOT revisit or deviate from these without explicit instruction.

### 11.1 Authentication
- **Stateless JWT.** No `refresh_tokens` table in the DB.
- Access token: `24h`. Refresh token: `7d`.
- JWT claims MUST contain: `user_id` (UUID), `role` (user_role enum value), `exp`.
- All admins log in via `POST /api/v1/auth/login`. The `role = 'admin'` claim in the JWT gates admin routes.
- Admin users are seeded manually via SQL. There is no admin self-registration endpoint.
- Admin routes are grouped under `/api/v1/admin/...` with a dedicated middleware that validates `role = 'admin'`.

### 11.2 Query Strategy
- **Pure `pgx/v5`** for all database access — no ORM, no external query builder.
- For endpoints with multiple optional filters (e.g., map search), use the **in-house `internal/pkg/querybuilder` helper** which tracks `$1, $2...` argument indexing automatically.
- For all other queries (simple lookups, inserts, updates), write raw SQL strings directly.
- For PostGIS (`ST_DWithin`, `ST_MakePoint`) and pgvector (`<=>` cosine operator) queries, always write raw SQL — no helper needed or appropriate.

### 11.3 Image Upload
- Phase 1: The client provides a `storage_url` string (uploaded externally). Backend stores the URL in `property_images.storage_url`.
- `image_hash` and `is_stock_flagged` columns will remain `NULL` in Phase 1.
- Phase 2: Presigned URL upload flow (backend generates presigned URL, client uploads directly to S3/Cloudinary, then notifies backend).

### 11.4 AI Photo Verification (pHash)
- Phase 1: When a landlord submits a listing, the system creates a `verifications` record with `type = 'ai_photo'` and `status = 'pending'`. This lands in the admin verification queue for manual review.
- Phase 2: When real image upload is implemented, compute pHash from image bytes, store in `image_hash`, and compare against a stock photo database to auto-approve or auto-reject.
- The schema is already prepared: `property_images.image_hash TEXT` and `property_images.is_stock_flagged BOOLEAN`.

### 11.5 Personality Embeddings (Squad Matching)
- The OpenAI `text-embedding-3-small` API is called **asynchronously** — never in the HTTP request cycle.
- When a user saves their Lifestyle Profile, the handler saves the data and sets a `pending_embeddings = TRUE` flag on the user row.
- A background goroutine worker pool polls for users with `pending_embeddings = TRUE`, calls OpenAI, stores the result in `users.personality_embedding`, and clears the flag.
- Squad matching uses pgvector cosine similarity (`<=>` operator). Only users with similarity **≥ 0.7** are returned.
- Match results are paginated at **10 per page**. The user manually selects who to invite.

### 11.6 Notifications
- Two delivery channels: **in-app** (stored in `notifications` table, polled via REST) and **email** (sent via SMTP on critical events).
- The `notifications` table has a `metadata JSONB` column for event context (e.g., `{"squad_id": "..."}`).
- Email is NOT a blocker for any feature — it is a best-effort side effect. If the email service fails, the in-app notification still persists.
- Email is sent for: KYC approved/rejected, property verified/rejected, token payment success, move-in confirmed.
- In-app notifications are created for all 13 `notification_type` ENUM values.

### 11.7 Payment Gateway
- **Full Razorpay integration** is the target for Phase 1.
- However, Razorpay is wired **last** — after Modules 1–7 (scaffold through squad system) are complete and stable.
- Until then, `POST /api/v1/transactions` creates the DB record (`status = 'initiated'`) and a stub endpoint `POST /api/v1/transactions/{id}/confirm` simulates the webhook for development testing.
- The Razorpay webhook endpoint (`POST /api/v1/transactions/webhook`) MUST verify the `X-Razorpay-Signature` header before processing any status update.

### 11.8 Build Order (Vertical Slice)
Modules are built end-to-end in this order. Each module produces working, testable endpoints before the next begins:

| # | Module | Key Deliverable | Status |
|---|--------|-----------------|--------|
| 1 | **Scaffold** | `GET /health` returns 200. DB connects. | ✅ |
| 2 | **Auth** | Register + login + refresh token | ✅ |
| 3 | **User Profile** | Lifestyle profile saved; embedding queued async | ✅ |
| 4 | **Landlord KYC** | Submit KYC; admin approve/reject | ✅ |
| 5 | **Properties & Rooms** | Create listing; map search with PostGIS | ✅ |
| 6 | **Verification Pipeline** | Admin verification queue; verified badge logic | ✅ |
| 7 | **Squad System** | Full squad lifecycle: lookup → match → invite → proposals | 🚧 |
| 8 | **Transactions & Payments** | Token payment flow; Razorpay integration | 🔲 |
| 9 | **Messages** | Squad private chat (PostgreSQL, paginated) | 🔲 |
| 10 | **Notifications** | In-app feed + email on critical events | 🔲 |
