package repository

import (
	"context"
	"database/sql"
	"errors"
	"namenotdecidedyet/internal/domain/transaction"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionRepo struct {
	pool *pgxpool.Pool
}

func NewTransactionRepo(pool *pgxpool.Pool) *TransactionRepo {
	return &TransactionRepo{pool: pool}
}

func (r *TransactionRepo) Create(ctx context.Context, tx *transaction.Transaction) error {
	const query = `
		INSERT INTO transactions (
			squad_id, user_id, property_id, type, amount, currency, status
		) VALUES ($1::UUID, $2::UUID, $3::UUID, $4::transaction_type, $5::DECIMAL, $6, $7::transaction_status)
		RETURNING id, created_at`

	return r.pool.QueryRow(ctx, query,
		tx.SquadID, tx.UserID, tx.PropertyID, tx.Type, tx.Amount, tx.Currency, tx.Status,
	).Scan(&tx.ID, &tx.CreatedAt)
}

func (r *TransactionRepo) UpdateGatewayInfo(ctx context.Context, id string, refID string, gateway string) error {
	const query = `
		UPDATE transactions 
		SET gateway_reference_id = $1, gateway = $2::payment_gateway
		WHERE id = $3::UUID`
	
	_, err := r.pool.Exec(ctx, query, refID, gateway, id)
	return err
}

func (r *TransactionRepo) GetByGatewayRef(ctx context.Context, refID string) (*transaction.Transaction, error) {
	const query = `
		SELECT id, squad_id, user_id, property_id, type, amount, currency, gateway, gateway_reference_id, gateway_status, status, created_at, settled_at
		FROM transactions
		WHERE gateway_reference_id = $1`

	var tx transaction.Transaction
	err := r.pool.QueryRow(ctx, query, refID).Scan(
		&tx.ID, &tx.SquadID, &tx.UserID, &tx.PropertyID, &tx.Type, &tx.Amount, &tx.Currency, &tx.Gateway, &tx.GatewayReferenceID, &tx.GatewayStatus, &tx.Status, &tx.CreatedAt, &tx.SettledAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	return &tx, err
}

func (r *TransactionRepo) MarkSuccess(ctx context.Context, id string, rawStatus string) error {
	const query = `
		UPDATE transactions 
		SET status = 'success', gateway_status = $1, settled_at = NOW()
		WHERE id = $2::UUID`
	
	_, err := r.pool.Exec(ctx, query, rawStatus, id)
	return err
}

func (r *TransactionRepo) GetTotalPaidForSquad(ctx context.Context, squadID string) (float64, error) {
	const query = `
		SELECT COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE squad_id = $1 AND status = 'success' AND type = 'token_payment'`
	
	var total float64
	err := r.pool.QueryRow(ctx, query, squadID).Scan(&total)
	return total, err
}

func (r *TransactionRepo) GetByUserID(ctx context.Context, userID string) ([]*transaction.Transaction, error) {
	const query = `
		SELECT id, squad_id, user_id, property_id, type, amount, currency, gateway, gateway_reference_id, gateway_status, status, created_at, settled_at
		FROM transactions
		WHERE user_id = $1::UUID
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*transaction.Transaction
	for rows.Next() {
		var tx transaction.Transaction
		if err := rows.Scan(
			&tx.ID, &tx.SquadID, &tx.UserID, &tx.PropertyID, &tx.Type, &tx.Amount,
			&tx.Currency, &tx.Gateway, &tx.GatewayReferenceID, &tx.GatewayStatus,
			&tx.Status, &tx.CreatedAt, &tx.SettledAt,
		); err != nil {
			return nil, err
		}
		results = append(results, &tx)
	}
	return results, rows.Err()
}
