package repository

import (
	"context"
	"errors"

	"namenotdecidedyet/internal/domain/verification"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VerificationRepo struct {
	pool *pgxpool.Pool
}

func NewVerificationRepo(pool *pgxpool.Pool) *VerificationRepo {
	return &VerificationRepo{pool: pool}
}

func (r *VerificationRepo) scanOne(row pgx.Row) (*verification.Verification, error) {
	var v verification.Verification
	err := row.Scan(
		&v.ID, &v.PropertyID, &v.AdminID, &v.VerificationType,
		&v.Status, &v.Notes, &v.VerifiedAt, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, verification.ErrVerificationNotFound
		}
		return nil, err
	}
	return &v, nil
}

func (r *VerificationRepo) CreateVerification(ctx context.Context, v *verification.Verification) (string, error) {
	const query = `
		INSERT INTO verifications (
			property_id, admin_id, verification_type, status, notes
		) VALUES (
			$1, $2, $3, $4, $5
		) RETURNING id`

	var id string
	err := r.pool.QueryRow(ctx, query,
		v.PropertyID, v.AdminID, v.VerificationType, v.Status, v.Notes,
	).Scan(&id)

	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *VerificationRepo) GetVerificationByID(ctx context.Context, id string) (*verification.Verification, error) {
	const query = `
		SELECT id, property_id, admin_id, verification_type, status, notes, verified_at, created_at, updated_at
		FROM   verifications
		WHERE  id = $1
		  AND  deleted_at IS NULL`

	return r.scanOne(r.pool.QueryRow(ctx, query, id))
}

func (r *VerificationRepo) UpdateVerification(ctx context.Context, v *verification.Verification) error {
	var query string
	var err error

	if v.Status == verification.StatusApproved {
		query = `
			UPDATE verifications
			SET    status = $1, admin_id = $2, notes = $3, verified_at = NOW()
			WHERE  id = $4 AND deleted_at IS NULL`
		_, err = r.pool.Exec(ctx, query, v.Status, v.AdminID, v.Notes, v.ID)
	} else {
		query = `
			UPDATE verifications
			SET    status = $1, admin_id = $2, notes = $3
			WHERE  id = $4 AND deleted_at IS NULL`
		_, err = r.pool.Exec(ctx, query, v.Status, v.AdminID, v.Notes, v.ID)
	}

	return err
}

func (r *VerificationRepo) GetVerificationsByProperty(ctx context.Context, propertyID string) ([]verification.Verification, error) {
	const query = `
		SELECT id, property_id, admin_id, verification_type, status, notes, verified_at, created_at, updated_at
		FROM   verifications
		WHERE  property_id = $1
		  AND  deleted_at IS NULL
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, query, propertyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []verification.Verification
	for rows.Next() {
		var v verification.Verification
		if err := rows.Scan(
			&v.ID, &v.PropertyID, &v.AdminID, &v.VerificationType,
			&v.Status, &v.Notes, &v.VerifiedAt, &v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, v)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}
