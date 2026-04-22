package kyc

import (
	"context"
	"errors"
	"fmt"

	"namenotdecidedyet/internal/domain/user"
	"namenotdecidedyet/internal/pkg/crypto"
)

// Repository defines the database operations required by the KYC Service.
type Repository interface {
	CreateKYC(ctx context.Context, k *LandlordKYC) (string, error)
	GetKYCByUserID(ctx context.Context, userID string) (*LandlordKYC, error)
	GetKYCByID(ctx context.Context, id string) (*LandlordKYC, error)
	UpdateStatus(ctx context.Context, id string, status string) error
	ListPending(ctx context.Context) ([]LandlordKYC, error)
}

// Service handles KYC business logic.
type Service struct {
	repo      Repository
	userRepo  user.Repository
	encryptor *crypto.Encryptor
}

// NewService creates a new KYC Service.
func NewService(repo Repository, userRepo user.Repository, encryptor *crypto.Encryptor) *Service {
	return &Service{
		repo:      repo,
		userRepo:  userRepo,
		encryptor: encryptor,
	}
}

// SubmitKYC securely encrypts the Aadhaar and PAN and creates a pending KYC record.
func (s *Service) SubmitKYC(ctx context.Context, userID string, input SubmitKYCInput) error {
	u, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.Role != user.RoleLandlord {
		return ErrOnlyLandlords
	}

	_, err = s.repo.GetKYCByUserID(ctx, userID)
	if err == nil {
		return ErrKYCAlreadyExists
	} else if !errors.Is(err, ErrKYCNotFound) {
		// some other DB error
		return err
	}

	encAadhaar, err := s.encryptor.Encrypt(input.Aadhaar)
	if err != nil {
		return fmt.Errorf("kyc service: failed to encrypt aadhaar: %w", err)
	}
	encPAN, err := s.encryptor.Encrypt(input.PAN)
	if err != nil {
		return fmt.Errorf("kyc service: failed to encrypt pan: %w", err)
	}

	k := &LandlordKYC{
		UserID:           userID,
		AadhaarEncrypted: &encAadhaar,
		PANEncrypted:     &encPAN,
		Status:           StatusPending,
	}

	_, err = s.repo.CreateKYC(ctx, k)
	return err
}

// GetMyStatus retrieves the current user's KYC submission status.
func (s *Service) GetMyStatus(ctx context.Context, userID string) (*LandlordKYC, error) {
	return s.repo.GetKYCByUserID(ctx, userID)
}

// ListPending retrieves all pending KYC submissions for admin review.
func (s *Service) ListPending(ctx context.Context) ([]LandlordKYC, error) {
	return s.repo.ListPending(ctx)
}

// ReviewKYC allows an admin to approve or reject a pending KYC submission.
func (s *Service) ReviewKYC(ctx context.Context, id string, input ReviewKYCInput) error {
	k, err := s.repo.GetKYCByID(ctx, id)
	if err != nil {
		return err
	}
	if k.Status != StatusPending {
		return ErrInvalidStatus
	}

	if err := s.repo.UpdateStatus(ctx, id, input.Status); err != nil {
		return err
	}

	// NOTE: Notification creation will be wired here in Module 10 (Notifications).
	return nil
}
