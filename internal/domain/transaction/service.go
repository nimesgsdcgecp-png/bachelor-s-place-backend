package transaction

import (
	"context"
	"fmt"
	"namenotdecidedyet/internal/domain/squad"
	"namenotdecidedyet/internal/pkg/payment"
)

// NotificationService is the interface the transaction service needs for event notifications.
type NotificationService interface {
	TokenPaymentSuccess(ctx context.Context, userID, squadID, propertyID string)
	MoveInConfirmed(ctx context.Context, userID, squadID, propertyID string)
}

type Service struct {
	repo         Repository
	squadRepo    SquadRepository
	propertyRepo PropertyRepository
	gateway      payment.Gateway
	notifier     NotificationService
}

func NewService(
	repo Repository,
	squadRepo SquadRepository,
	propertyRepo PropertyRepository,
	gateway payment.Gateway,
	notifier NotificationService,
) *Service {
	return &Service{
		repo:         repo,
		squadRepo:    squadRepo,
		propertyRepo: propertyRepo,
		gateway:      gateway,
		notifier:     notifier,
	}
}

// InitiateTokenPayment starts the payment process for a squad.
// BR-09: Transaction record is ALWAYS created before the gateway call.
func (s *Service) InitiateTokenPayment(ctx context.Context, userID, squadID string) (*Transaction, *payment.Order, error) {
	sq, err := s.squadRepo.GetSquadByID(ctx, squadID)
	if err != nil {
		return nil, nil, err
	}

	if sq.PropertyID == nil {
		return nil, nil, fmt.Errorf("squad has no property selected")
	}

	prop, err := s.propertyRepo.GetPropertyByID(ctx, *sq.PropertyID)
	if err != nil {
		return nil, nil, err
	}

	// Calculate amount based on payment model
	depositAmount := 0.0
	if prop.DepositAmount != nil {
		depositAmount = *prop.DepositAmount
	}

	var amount float64
	if sq.PaymentModel == squad.PaymentModelLeaderPaysAll {
		if userID != sq.CreatedBy {
			return nil, nil, fmt.Errorf("only the squad leader can initiate payment in 'leader_pays_all' mode")
		}
		amount = depositAmount
	} else {
		// Split evenly across all accepted members
		amount = depositAmount / float64(sq.CurrentMemberCount)
	}

	// BR-09: Create the DB record first before calling gateway
	tx := &Transaction{
		SquadID:    &squadID,
		UserID:     userID,
		PropertyID: *sq.PropertyID,
		Type:       TypeTokenPayment,
		Amount:     amount,
		Currency:   "INR",
		Status:     StatusInitiated,
	}
	if err := s.repo.Create(ctx, tx); err != nil {
		return nil, nil, err
	}

	// Create order on gateway
	metadata := map[string]string{
		"transaction_id": tx.ID,
		"squad_id":       squadID,
	}
	order, err := s.gateway.CreateOrder(ctx, amount, "INR", metadata)
	if err != nil {
		// Record stays 'initiated' — can be retried
		return nil, nil, err
	}

	// Persist gateway reference — use gateway.GatewayName() (NOT hardcoded "mock")
	if err := s.repo.UpdateGatewayInfo(ctx, tx.ID, order.ID, s.gateway.GatewayName()); err != nil {
		return nil, nil, err
	}

	tx.GatewayReferenceID = &order.ID
	return tx, order, nil
}

// ProcessWebhook handles a verified callback from the payment gateway.
// The handler is responsible for passing the raw body and signature header.
// BR: Webhook handler always returns 200 to the gateway to prevent retries — caller must handle that.
func (s *Service) ProcessWebhook(ctx context.Context, rawBody []byte, signature string) error {
	// 1. Verify signature (HMAC-SHA256 for Razorpay)
	ok, err := s.gateway.VerifyWebhook(rawBody, signature)
	if err != nil || !ok {
		return fmt.Errorf("invalid webhook signature")
	}

	// 2. Extract the gateway_reference_id from the body
	// Razorpay webhook payload: {"payload": {"payment": {"entity": {"order_id": "..."}}}}
	// We store the order_id as our gateway_reference_id
	gatewayRefID, err := extractOrderIDFromWebhook(rawBody)
	if err != nil {
		return fmt.Errorf("webhook: could not extract order id: %w", err)
	}

	// 3. Look up our transaction
	tx, err := s.repo.GetByGatewayRef(ctx, gatewayRefID)
	if err != nil {
		return err
	}

	if tx.Status == StatusSuccess {
		return nil // Idempotent — already processed
	}

	// 4. Mark as success
	if err := s.repo.MarkSuccess(ctx, tx.ID, "paid"); err != nil {
		return err
	}

	// 5. Update squad deposit and check if fully paid
	if tx.SquadID != nil {
		if err := s.squadRepo.UpdateTotalDeposit(ctx, *tx.SquadID, tx.Amount); err != nil {
			return err
		}

		sq, err := s.squadRepo.GetSquadByID(ctx, *tx.SquadID)
		if err != nil {
			return err
		}

		prop, err := s.propertyRepo.GetPropertyByID(ctx, *sq.PropertyID)
		if err != nil {
			return err
		}

		depositRequired := 0.0
		if prop.DepositAmount != nil {
			depositRequired = *prop.DepositAmount
		}

		// Lock the squad when deposit is fully collected
		if sq.TotalDepositCollected >= depositRequired-0.01 {
			if err := s.squadRepo.SetStatusLocked(ctx, *tx.SquadID); err != nil {
				return err
			}
			// Notify the paying user — token payment success
			if s.notifier != nil {
				s.notifier.TokenPaymentSuccess(ctx, tx.UserID, *tx.SquadID, tx.PropertyID)
			}
		}
	}

	return nil
}

// GetTransactionHistory returns all transactions for a user (newest first).
func (s *Service) GetTransactionHistory(ctx context.Context, userID string) ([]*Transaction, error) {
	return s.repo.GetByUserID(ctx, userID)
}

// ConfirmMoveIn is called by the squad leader to confirm the squad has physically moved in.
// It transitions the squad to 'moved_in', marks the property as 'occupied',
// and creates a success_fee transaction record for future landlord payout processing.
func (s *Service) ConfirmMoveIn(ctx context.Context, userID, squadID string) error {
	sq, err := s.squadRepo.GetSquadByID(ctx, squadID)
	if err != nil {
		return err
	}

	// Only the leader can confirm move-in
	if userID != sq.CreatedBy {
		return fmt.Errorf("only the squad leader can confirm move-in")
	}

	// Squad must be locked (token paid) before move-in can be confirmed
	if sq.Status != squad.StatusLocked {
		return fmt.Errorf("squad must be in 'locked' status to confirm move-in (current: %s)", sq.Status)
	}

	if sq.PropertyID == nil {
		return fmt.Errorf("squad has no property associated")
	}

	// 1. Create a success_fee transaction record (for future payout processing)
	successFeeTx := &Transaction{
		SquadID:    &squadID,
		UserID:     userID,
		PropertyID: *sq.PropertyID,
		Type:       TypeSuccessFee,
		Amount:     0, // Amount set when admin processes the payout
		Currency:   "INR",
		Status:     StatusInitiated,
	}
	if err := s.repo.Create(ctx, successFeeTx); err != nil {
		return fmt.Errorf("failed to create success_fee record: %w", err)
	}

	// 2. Transition squad → moved_in
	if err := s.squadRepo.SetStatusMovedIn(ctx, squadID); err != nil {
		return err
	}

	// 3. Mark property → occupied
	if err := s.propertyRepo.SetPropertyOccupied(ctx, *sq.PropertyID); err != nil {
		return err
	}

	// Notify the leader — move-in confirmed
	if s.notifier != nil {
		s.notifier.MoveInConfirmed(ctx, userID, squadID, *sq.PropertyID)
	}

	return nil
}
