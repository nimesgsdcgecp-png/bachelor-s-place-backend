package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Notification is the data model for the notifications table.
type Notification struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	IsRead    bool            `json:"is_read"`
	ReadAt    *time.Time      `json:"read_at,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"` // JSONB — passed through as-is
	CreatedAt time.Time       `json:"created_at"`
}

type NotificationRepo struct {
	pool *pgxpool.Pool
}

func NewNotificationRepo(pool *pgxpool.Pool) *NotificationRepo {
	return &NotificationRepo{pool: pool}
}

// Create inserts a new notification for a user.
// metadata is an optional JSON object, e.g. {"squad_id": "...", "property_id": "..."}.
func (r *NotificationRepo) Create(ctx context.Context, userID, notifType, title, body string, metadata map[string]string) (*Notification, error) {
	var metaJSON []byte
	if len(metadata) > 0 {
		var err error
		metaJSON, err = json.Marshal(metadata)
		if err != nil {
			return nil, err
		}
	}

	const query = `
		INSERT INTO notifications (user_id, type, title, body, metadata)
		VALUES ($1::UUID, $2::notification_type, $3, $4, $5::JSONB)
		RETURNING id, user_id, type, title, body, is_read, read_at, metadata, created_at`

	var n Notification
	err := r.pool.QueryRow(ctx, query, userID, notifType, title, body, metaJSON).Scan(
		&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.IsRead, &n.ReadAt, &n.Metadata, &n.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// GetForUser returns paginated notifications for a user — unread first, then newest first.
// Supports offset pagination via `page` (1-indexed) and `perPage`.
func (r *NotificationRepo) GetForUser(ctx context.Context, userID string, page, perPage int) ([]*Notification, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 || perPage > 50 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	// Count total for pagination metadata
	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1::UUID AND deleted_at IS NULL`,
		userID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	const query = `
		SELECT id, user_id, type, title, body, is_read, read_at, metadata, created_at
		FROM notifications
		WHERE user_id = $1::UUID AND deleted_at IS NULL
		ORDER BY is_read ASC, created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.IsRead, &n.ReadAt, &n.Metadata, &n.CreatedAt); err != nil {
			return nil, 0, err
		}
		results = append(results, &n)
	}
	return results, total, rows.Err()
}

// MarkOneRead marks a single notification as read for the given user.
// Returns ErrNoRows if not found or doesn't belong to user.
func (r *NotificationRepo) MarkOneRead(ctx context.Context, notifID, userID string) error {
	const query = `
		UPDATE notifications
		SET is_read = TRUE, read_at = NOW()
		WHERE id = $1::UUID AND user_id = $2::UUID AND deleted_at IS NULL`

	cmd, err := r.pool.Exec(ctx, query, notifID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// MarkAllRead marks every unread notification as read for the given user.
func (r *NotificationRepo) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	const query = `
		UPDATE notifications
		SET is_read = TRUE, read_at = NOW()
		WHERE user_id = $1::UUID AND is_read = FALSE AND deleted_at IS NULL`

	cmd, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return 0, err
	}
	return cmd.RowsAffected(), nil
}

// GetUserEmail looks up a user's email — used to send email notifications.
func (r *NotificationRepo) GetUserEmail(ctx context.Context, userID string) (string, error) {
	var email string
	err := r.pool.QueryRow(ctx,
		`SELECT email FROM users WHERE id = $1::UUID AND deleted_at IS NULL`,
		userID,
	).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return email, err
}
