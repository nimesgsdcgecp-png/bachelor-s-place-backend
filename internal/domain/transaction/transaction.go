package transaction

import (
	"context"
	"time"

	"namenotdecidedyet/internal/domain/property"
	"namenotdecidedyet/internal/domain/squad"
)

type TransactionType string

const (
	TypeTokenPayment TransactionType = "token_payment"
	TypeSuccessFee   TransactionType = "success_fee"
	TypeRefund       TransactionType = "refund"
	TypePayout       TransactionType = "payout"
)

type TransactionStatus string

const (
	StatusInitiated TransactionStatus = "initiated"
	StatusSuccess   TransactionStatus = "success"
	StatusFailed    TransactionStatus = "failed"
	StatusRefunded  TransactionStatus = "refunded"
)

type Transaction struct {
	ID                 string            `json:"id"`
	SquadID            *string           `json:"squad_id,omitempty"`
	UserID             string            `json:"user_id"`
	PropertyID         string            `json:"property_id"`
	Type               TransactionType   `json:"type"`
	Amount             float64           `json:"amount"`
	Currency           string            `json:"currency"`
	Gateway            *string           `json:"gateway,omitempty"`
	GatewayReferenceID *string           `json:"gateway_reference_id,omitempty"`
	GatewayStatus      *string           `json:"gateway_status,omitempty"`
	Status             TransactionStatus `json:"status"`
	CreatedAt          time.Time         `json:"created_at"`
	SettledAt          *time.Time        `json:"settled_at,omitempty"`
}

// Repository defines the interface for transaction data operations.
type Repository interface {
	Create(ctx context.Context, tx *Transaction) error
	UpdateGatewayInfo(ctx context.Context, id string, refID string, gateway string) error
	GetByGatewayRef(ctx context.Context, refID string) (*Transaction, error)
	GetByUserID(ctx context.Context, userID string) ([]*Transaction, error)
	MarkSuccess(ctx context.Context, id string, rawStatus string) error
	GetTotalPaidForSquad(ctx context.Context, squadID string) (float64, error)
}

// Cross-domain interfaces needed by the transaction service.
type SquadRepository interface {
	GetSquadByID(ctx context.Context, id string) (*squad.Squad, error)
	UpdateTotalDeposit(ctx context.Context, squadID string, amount float64) error
	SetStatusLocked(ctx context.Context, squadID string) error
	SetStatusMovedIn(ctx context.Context, squadID string) error
}

type PropertyRepository interface {
	GetPropertyByID(ctx context.Context, id string) (*property.Property, error)
	SetPropertyOccupied(ctx context.Context, propertyID string) error
}
