// Package repository provides all PostgreSQL data access implementations.
// All queries use named columns — never SELECT *.
// All queries filter deleted_at IS NULL (soft-delete convention).
// Raw pgx errors are never returned to callers — they are mapped to domain errors.
package repository

import (
	"context"
	"errors"

	"namenotdecidedyet/internal/domain/user"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepo handles all database operations for the users table.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo creates a new UserRepo backed by the given connection pool.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// CreateUser inserts a new user and returns the generated UUID.
// Returns user.ErrEmailAlreadyExists if the email violates the UNIQUE constraint.
func (r *UserRepo) CreateUser(ctx context.Context, u *user.User) (string, error) {
	const query = `
		INSERT INTO users (
			name, email, password_hash, role,
			lifestyle_tags, preferred_localities
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`

	var id string
	err := r.pool.QueryRow(ctx, query,
		u.Name,
		u.Email,
		u.PasswordHash,
		u.Role,
		u.LifestyleTags,
		u.PreferredLocalities,
	).Scan(&id)

	if err != nil {
		if isUniqueViolation(err) {
			return "", user.ErrEmailAlreadyExists
		}
		return "", err
	}
	return id, nil
}

// GetUserByEmail retrieves a non-deleted user by email address.
// Returns user.ErrUserNotFound when no row matches.
func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*user.User, error) {
	const query = `
		SELECT id, name, email, password_hash, role,
		       lifestyle_tags, preferred_localities,
		       bio, budget_min, budget_max, pending_embeddings,
		       is_active, created_at, updated_at
		FROM   users
		WHERE  email = $1
		  AND  deleted_at IS NULL`

	return r.scanOne(r.pool.QueryRow(ctx, query, email))
}

// GetUserByID retrieves a non-deleted user by UUID.
// Returns user.ErrUserNotFound when no row matches.
func (r *UserRepo) GetUserByID(ctx context.Context, id string) (*user.User, error) {
	const query = `
		SELECT id, name, email, password_hash, role,
		       lifestyle_tags, preferred_localities,
		       bio, budget_min, budget_max, pending_embeddings,
		       is_active, created_at, updated_at
		FROM   users
		WHERE  id = $1
		  AND  deleted_at IS NULL`

	return r.scanOne(r.pool.QueryRow(ctx, query, id))
}

// scanOne maps a single pgx.Row into a *user.User.
func (r *UserRepo) scanOne(row pgx.Row) (*user.User, error) {
	u := &user.User{}
	err := row.Scan(
		&u.ID,
		&u.Name,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.LifestyleTags,
		&u.PreferredLocalities,
		&u.Bio,
		&u.BudgetMin,
		&u.BudgetMax,
		&u.PendingEmbeddings,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrUserNotFound
		}
		return nil, err
	}
	return u, nil
}

// isUniqueViolation reports whether a pgx error is a PostgreSQL unique violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// UpdateProfile updates the lifestyle details of a user and sets pending_embeddings to TRUE.
func (r *UserRepo) UpdateProfile(ctx context.Context, userID string, input user.UpdateProfileInput) error {
	const query = `
		UPDATE users
		SET    lifestyle_tags = $1,
		       bio = $2,
		       budget_min = $3,
		       budget_max = $4,
		       preferred_localities = $5,
		       pending_embeddings = TRUE
		WHERE  id = $6
		  AND  deleted_at IS NULL`

	cmd, err := r.pool.Exec(ctx, query,
		input.LifestyleTags,
		input.Bio,
		input.BudgetMin,
		input.BudgetMax,
		input.PreferredLocalities,
		userID,
	)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return user.ErrUserNotFound
	}
	return nil
}
