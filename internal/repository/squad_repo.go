package repository

import (
	"context"
	"errors"
	"time"

	"namenotdecidedyet/internal/domain/squad"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SquadRepo struct {
	pool *pgxpool.Pool
}

func NewSquadRepo(pool *pgxpool.Pool) *SquadRepo {
	return &SquadRepo{pool: pool}
}

// CreateLookup registers a tenant's intent to find a squad.
func (r *SquadRepo) CreateLookup(ctx context.Context, l *squad.SquadLookup) (string, error) {
	const query = `
		INSERT INTO squad_lookups (
			user_id, property_id, locality_preference, budget_min, budget_max, status
		) VALUES ($1, $2, $3, $4, $5, 'active')
		RETURNING id`

	var id string
	err := r.pool.QueryRow(ctx, query,
		l.UserID, l.PropertyID, l.LocalityPreference, l.BudgetMin, l.BudgetMax,
	).Scan(&id)

	return id, err
}

func (r *SquadRepo) GetActiveLookup(ctx context.Context, userID string) (*squad.SquadLookup, error) {
	const query = `
		SELECT id, user_id, property_id, locality_preference, budget_min, budget_max, status, created_at, expires_at
		FROM   squad_lookups
		WHERE  user_id = $1 AND status = 'active' AND deleted_at IS NULL`

	var l squad.SquadLookup
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&l.ID, &l.UserID, &l.PropertyID, &l.LocalityPreference, &l.BudgetMin, &l.BudgetMax,
		&l.Status, &l.CreatedAt, &l.ExpiresAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, squad.ErrLookupNotFound
		}
		return nil, err
	}
	return &l, nil
}

func (r *SquadRepo) DeleteLookup(ctx context.Context, userID string) error {
	const query = `UPDATE squad_lookups SET status = 'inactive', deleted_at = NOW() WHERE user_id = $1 AND status = 'active'`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

// FindMatches uses pgvector cosine similarity to find compatible users.
// Only returns users with a similarity score >= 0.7.
func (r *SquadRepo) FindMatches(ctx context.Context, userID string, limit int, offset int) ([]squad.MatchResult, error) {
	const query = `
		SELECT 
			u.id, u.name, u.lifestyle_tags, u.bio,
			1 - (u.personality_embedding <=> (SELECT personality_embedding FROM users WHERE id = $1)) AS compatibility_score
		FROM users u
		JOIN squad_lookups sl ON sl.user_id = u.id
		WHERE u.id != $1
		  AND u.deleted_at IS NULL
		  AND sl.status = 'active'
		  AND u.personality_embedding IS NOT NULL
		  AND 1 - (u.personality_embedding <=> (SELECT personality_embedding FROM users WHERE id = $1)) >= 0.7
		ORDER BY compatibility_score DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []squad.MatchResult
	for rows.Next() {
		var m squad.MatchResult
		if err := rows.Scan(&m.UserID, &m.Name, &m.LifestyleTags, &m.Bio, &m.CompatibilityScore); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, nil
}

// CreateSquad creates a new squad and adds the creator as the leader.
func (r *SquadRepo) CreateSquad(ctx context.Context, s *squad.Squad, leaderID string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	const squadQuery = `
		INSERT INTO squads (property_id, room_id, name, status, payment_model, max_size, current_member_count, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, 1, $7)
		RETURNING id`

	var squadID string
	err = tx.QueryRow(ctx, squadQuery,
		s.PropertyID, s.RoomID, s.Name, s.Status, s.PaymentModel, s.MaxSize, s.CreatedBy,
	).Scan(&squadID)
	if err != nil {
		return "", err
	}

	const memberQuery = `
		INSERT INTO squad_members (squad_id, user_id, role, status, joined_at)
		VALUES ($1, $2, 'leader', 'accepted', NOW())`

	_, err = tx.Exec(ctx, memberQuery, squadID, leaderID)
	if err != nil {
		return "", err
	}

	return squadID, tx.Commit(ctx)
}

func (r *SquadRepo) GetSquadByID(ctx context.Context, id string) (*squad.Squad, error) {
	const query = `
		SELECT id, property_id, room_id, name, status, payment_model, max_size, current_member_count, created_by, total_deposit_collected, token_paid_at, created_at, updated_at
		FROM   squads
		WHERE  id = $1 AND deleted_at IS NULL`

	var s squad.Squad
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.PropertyID, &s.RoomID, &s.Name, &s.Status, &s.PaymentModel, &s.MaxSize, &s.CurrentMemberCount,
		&s.CreatedBy, &s.TotalDepositCollected, &s.TokenPaidAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, squad.ErrSquadNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *SquadRepo) GetMembers(ctx context.Context, squadID string) ([]squad.SquadMember, error) {
	const query = `
		SELECT sm.id, sm.squad_id, sm.user_id, u.name, sm.role, sm.status, sm.share_amount, sm.joined_at, sm.created_at
		FROM   squad_members sm
		JOIN   users u ON sm.user_id = u.id
		WHERE  sm.squad_id = $1 AND sm.deleted_at IS NULL`

	rows, err := r.pool.Query(ctx, query, squadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []squad.SquadMember
	for rows.Next() {
		var m squad.SquadMember
		if err := rows.Scan(
			&m.ID, &m.SquadID, &m.UserID, &m.UserName, &m.Role, &m.Status, &m.ShareAmount, &m.JoinedAt, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

func (r *SquadRepo) AddMember(ctx context.Context, squadID, userID string, role squad.MemberRole, status squad.MemberStatus) error {
	const query = `
		INSERT INTO squad_members (squad_id, user_id, role, status)
		VALUES ($1, $2, $3, $4)`
	_, err := r.pool.Exec(ctx, query, squadID, userID, role, status)
	return err
}

func (r *SquadRepo) UpdateMemberStatus(ctx context.Context, squadID, userID string, status squad.MemberStatus) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var joinedAt *time.Time
	if status == squad.MemberStatusAccepted {
		now := time.Now()
		joinedAt = &now
	}

	const updateMember = `
		UPDATE squad_members 
		SET    status = $1, joined_at = COALESCE(joined_at, $2)
		WHERE  squad_id = $3 AND user_id = $4`

	_, err = tx.Exec(ctx, updateMember, status, joinedAt, squadID, userID)
	if err != nil {
		return err
	}

	// If accepted, increment member count
	if status == squad.MemberStatusAccepted {
		const updateCount = `UPDATE squads SET current_member_count = current_member_count + 1 WHERE id = $1`
		_, err = tx.Exec(ctx, updateCount, squadID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *SquadRepo) RemoveMember(ctx context.Context, squadID, userID string) error {
	const query = `UPDATE squad_members SET status = 'left', deleted_at = NOW() WHERE squad_id = $1 AND user_id = $2`
	_, err := r.pool.Exec(ctx, query, squadID, userID)
	return err
}

func (r *SquadRepo) CreateProposal(ctx context.Context, squadID, userID, propertyID string, roomID *string) (string, error) {
	const query = `
		INSERT INTO squad_property_proposals (squad_id, proposed_by, property_id, room_id, status)
		VALUES ($1, $2, $3, $4, 'pending')
		RETURNING id`
	var id string
	err := r.pool.QueryRow(ctx, query, squadID, userID, propertyID, roomID).Scan(&id)
	return id, err
}

func (r *SquadRepo) GetProposals(ctx context.Context, squadID string) ([]map[string]interface{}, error) {
	const query = `
		SELECT spp.id, spp.property_id, p.title, spp.proposed_by, u.name as proposer_name, spp.status, spp.proposed_at
		FROM   squad_property_proposals spp
		JOIN   properties p ON spp.property_id = p.id
		JOIN   users u ON spp.proposed_by = u.id
		WHERE  spp.squad_id = $1 AND spp.deleted_at IS NULL`

	rows, err := r.pool.Query(ctx, query, squadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, propID, propTitle, propBy, propByName, status string
		var proposedAt time.Time
		if err := rows.Scan(&id, &propID, &propTitle, &propBy, &propByName, &status, &proposedAt); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":            id,
			"property_id":   propID,
			"property_name": propTitle,
			"proposed_by":   propBy,
			"proposer_name": propByName,
			"status":        status,
			"proposed_at":   proposedAt,
		})
	}
	return results, nil
}

func (r *SquadRepo) ResolveProposal(ctx context.Context, proposalID string, status string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	const updateProp = `UPDATE squad_property_proposals SET status = $1, resolved_at = NOW() WHERE id = $2 RETURNING squad_id, property_id, room_id`
	var squadID, propertyID string
	var roomID *string
	err = tx.QueryRow(ctx, updateProp, status, proposalID).Scan(&squadID, &propertyID, &roomID)
	if err != nil {
		return err
	}

	if status == "accepted" {
		// Update squad status and property
		const updateSquad = `UPDATE squads SET status = 'forming', property_id = $1, room_id = $2 WHERE id = $3`
		_, err = tx.Exec(ctx, updateSquad, propertyID, roomID, squadID)
		if err != nil {
			return err
		}

		// Reject all other pending proposals for this squad (FR-4.4)
		const rejectOthers = `UPDATE squad_property_proposals SET status = 'rejected', resolved_at = NOW() WHERE squad_id = $1 AND id != $2 AND status = 'pending'`
		_, err = tx.Exec(ctx, rejectOthers, squadID, proposalID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
func (r *SquadRepo) UpdateTotalDeposit(ctx context.Context, squadID string, amount float64) error {
	const query = `UPDATE squads SET total_deposit_collected = total_deposit_collected + $1, updated_at = NOW() WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, amount, squadID)
	return err
}

func (r *SquadRepo) SetStatusLocked(ctx context.Context, squadID string) error {
	const query = `UPDATE squads SET status = 'locked', token_paid_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, squadID)
	return err
}

func (r *SquadRepo) SetStatusMovedIn(ctx context.Context, squadID string) error {
	const query = `UPDATE squads SET status = 'moved_in', move_in_confirmed_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, squadID)
	return err
}

func (r *SquadRepo) GetLandlordContact(ctx context.Context, propertyID string) (map[string]string, error) {
	const query = `
		SELECT u.name, u.phone_encrypted
		FROM   properties p
		JOIN   users u ON p.owner_id = u.id
		WHERE  p.id = $1`
	
	var name, phone string
	err := r.pool.QueryRow(ctx, query, propertyID).Scan(&name, &phone)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"name":  name,
		"phone": phone,
	}, nil
}
