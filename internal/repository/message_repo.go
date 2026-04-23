package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Message is a lightweight struct used only by the message repo.
// It lives here to avoid a full domain package for a simple CRUD resource.
type Message struct {
	ID          string    `json:"id"`
	SquadID     string    `json:"squad_id"`
	SenderID    string    `json:"sender_id"`
	SenderName  string    `json:"sender_name,omitempty"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type"`
	SentAt      time.Time `json:"sent_at"`
	ReadBy      []string  `json:"read_by"`
}

type MessageRepo struct {
	pool *pgxpool.Pool
}

func NewMessageRepo(pool *pgxpool.Pool) *MessageRepo {
	return &MessageRepo{pool: pool}
}

// CreateMessage inserts a new message into the squad chat.
func (r *MessageRepo) CreateMessage(ctx context.Context, squadID, senderID, content, contentType string) (*Message, error) {
	const query = `
		INSERT INTO messages (squad_id, sender_id, content, content_type, sent_at, read_by)
		VALUES ($1::UUID, $2::UUID, $3, $4::message_content_type, NOW(), ARRAY[]::UUID[])
		RETURNING id, squad_id, sender_id, content, content_type, sent_at, read_by`

	var m Message
	err := r.pool.QueryRow(ctx, query, squadID, senderID, content, contentType).Scan(
		&m.ID, &m.SquadID, &m.SenderID, &m.Content, &m.ContentType, &m.SentAt, &m.ReadBy,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetMessages returns paginated messages for a squad using cursor-based pagination.
// cursor is a sent_at timestamp string (RFC3339). Pass "" to get the most recent messages.
// Returns up to `limit` messages ordered by sent_at DESC.
func (r *MessageRepo) GetMessages(ctx context.Context, squadID string, cursor string, limit int) ([]*Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 30
	}

	baseQuery := `
		SELECT m.id, m.squad_id, m.sender_id, u.name, m.content, m.content_type, m.sent_at, m.read_by
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		WHERE m.squad_id = $1::UUID AND m.deleted_at IS NULL`

	var rows pgx.Rows
	var err error

	if cursor == "" {
		rows, err = r.pool.Query(ctx, baseQuery+" ORDER BY m.sent_at DESC LIMIT $2", squadID, limit)
	} else {
		rows, err = r.pool.Query(ctx, baseQuery+" AND m.sent_at < $2::TIMESTAMPTZ ORDER BY m.sent_at DESC LIMIT $3", squadID, cursor, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SquadID, &m.SenderID, &m.SenderName, &m.Content, &m.ContentType, &m.SentAt, &m.ReadBy); err != nil {
			return nil, err
		}
		messages = append(messages, &m)
	}
	return messages, rows.Err()
}

// MarkRead appends the userID to the read_by array for all messages in the squad
// that were sent before (and including) the given cursor timestamp.
// Safe for squads ≤ 5 members — uses array_append.
func (r *MessageRepo) MarkRead(ctx context.Context, squadID, userID string) (int64, error) {
	const query = `
		UPDATE messages
		SET read_by = array_append(read_by, $1::UUID)
		WHERE squad_id = $2::UUID
		  AND deleted_at IS NULL
		  AND NOT ($1::UUID = ANY(read_by))`

	cmd, err := r.pool.Exec(ctx, query, userID, squadID)
	if err != nil {
		return 0, err
	}
	return cmd.RowsAffected(), nil
}

// IsMember checks if a user is an accepted member of a squad.
// Required gate before chat access.
func (r *MessageRepo) IsMember(ctx context.Context, squadID, userID string) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM squad_members
			WHERE squad_id = $1::UUID AND user_id = $2::UUID AND status = 'accepted' AND deleted_at IS NULL
		)`
	var ok bool
	err := r.pool.QueryRow(ctx, query, squadID, userID).Scan(&ok)
	return ok, err
}
