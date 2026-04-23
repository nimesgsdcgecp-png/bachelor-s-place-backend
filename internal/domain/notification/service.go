package notification

import (
	"context"

	"namenotdecidedyet/internal/pkg/email"
	"namenotdecidedyet/internal/repository"

	"github.com/rs/zerolog/log"
)

// Service provides a single Fire-and-notify API used by other domain services.
// All operations are best-effort — failures are logged, never propagated.
type Service struct {
	repo   *repository.NotificationRepo
	mailer *email.Mailer
}

func NewService(repo *repository.NotificationRepo, mailer *email.Mailer) *Service {
	return &Service{repo: repo, mailer: mailer}
}

// Notify creates an in-app notification and optionally sends an email.
// sendEmail controls whether an email is dispatched (best-effort, non-blocking).
func (s *Service) Notify(ctx context.Context, userID, notifType, title, body string, metadata map[string]string, sendEmail bool) {
	// 1. Persist in-app notification
	_, err := s.repo.Create(ctx, userID, notifType, title, body, metadata)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Str("type", notifType).Msg("notification: failed to create")
		return
	}

	// 2. Optionally fire email (non-blocking goroutine inside Mailer.Send)
	if sendEmail {
		emailAddr, err := s.repo.GetUserEmail(ctx, userID)
		if err != nil || emailAddr == "" {
			log.Warn().Str("user_id", userID).Msg("notification: could not fetch email for user")
			return
		}
		s.mailer.Send(emailAddr, title, body)
	}
}

// TokenPaymentSuccess notifies squad members after a token payment is confirmed.
func (s *Service) TokenPaymentSuccess(ctx context.Context, userID, squadID, propertyID string) {
	s.Notify(ctx, userID,
		"token_payment_success",
		"Token Payment Confirmed! 🎉",
		"Your token payment was successful. The property is now locked for your squad.",
		map[string]string{"squad_id": squadID, "property_id": propertyID},
		true,
	)
}

// MoveInConfirmed notifies the squad leader after move-in is confirmed.
func (s *Service) MoveInConfirmed(ctx context.Context, userID, squadID, propertyID string) {
	s.Notify(ctx, userID,
		"move_in_confirmed",
		"Move-In Confirmed! 🏠",
		"Your move-in has been confirmed. Welcome to your new home!",
		map[string]string{"squad_id": squadID, "property_id": propertyID},
		true,
	)
}

// SquadInvite notifies a user they've been invited to a squad.
func (s *Service) SquadInvite(ctx context.Context, userID, squadID, inviterName string) {
	s.Notify(ctx, userID,
		"squad_invite",
		"Squad Invitation Received",
		inviterName+" has invited you to join their squad.",
		map[string]string{"squad_id": squadID},
		false,
	)
}

// ProposalAccepted notifies a user their property proposal was accepted.
func (s *Service) ProposalAccepted(ctx context.Context, userID, squadID, propertyID string) {
	s.Notify(ctx, userID,
		"proposal_accepted",
		"Property Proposal Accepted ✅",
		"The squad leader accepted your property proposal. Time to pay the token!",
		map[string]string{"squad_id": squadID, "property_id": propertyID},
		false,
	)
}

// KYCApproved notifies a landlord their KYC was approved.
func (s *Service) KYCApproved(ctx context.Context, userID string) {
	s.Notify(ctx, userID,
		"kyc_approved",
		"KYC Approved ✅",
		"Your identity verification has been approved. You can now list properties on BachelorPad.",
		nil,
		true,
	)
}

// KYCRejected notifies a landlord their KYC was rejected.
func (s *Service) KYCRejected(ctx context.Context, userID, reason string) {
	s.Notify(ctx, userID,
		"kyc_rejected",
		"KYC Rejected",
		"Your identity verification was rejected. Reason: "+reason+". Please resubmit with correct documents.",
		nil,
		true,
	)
}

// PropertyVerified notifies a landlord their property listing is now verified.
func (s *Service) PropertyVerified(ctx context.Context, userID, propertyID string) {
	s.Notify(ctx, userID,
		"property_verified",
		"Property Verified ✅",
		"Your property listing has been verified and is now visible to tenants.",
		map[string]string{"property_id": propertyID},
		true,
	)
}
