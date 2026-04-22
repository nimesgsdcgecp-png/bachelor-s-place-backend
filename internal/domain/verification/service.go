package verification

import (
	"context"
	"fmt"

	"namenotdecidedyet/internal/domain/property"
)

// Repository defines database operations for verifications.
type Repository interface {
	CreateVerification(ctx context.Context, v *Verification) (string, error)
	GetVerificationByID(ctx context.Context, id string) (*Verification, error)
	UpdateVerification(ctx context.Context, v *Verification) error
	GetVerificationsByProperty(ctx context.Context, propertyID string) ([]Verification, error)
}

// Service manages the property verification pipeline.
type Service struct {
	repo         Repository
	propertyRepo property.Repository
}

// NewService creates a new Verification Service.
func NewService(repo Repository, propertyRepo property.Repository) *Service {
	return &Service{
		repo:         repo,
		propertyRepo: propertyRepo,
	}
}

// InitiateVerification creates a new verification record for a property.
func (s *Service) InitiateVerification(ctx context.Context, propertyID string, adminID string, input CreateVerificationInput) (string, error) {
	// Ensure property exists
	p, err := s.propertyRepo.GetPropertyByID(ctx, propertyID)
	if err != nil {
		return "", fmt.Errorf("verification service: failed to get property: %w", err)
	}
	if p.Status == property.StatusVerified {
		return "", ErrPropertyAlreadyVerified
	}

	// Move property to pending_verification if it was still a draft
	if p.Status == property.StatusDraft {
		if err := s.propertyRepo.UpdateStatus(ctx, propertyID, property.StatusPendingVerification); err != nil {
			return "", err
		}
	}

	v := &Verification{
		PropertyID:       propertyID,
		AdminID:          &adminID,
		VerificationType: input.VerificationType,
		Status:           StatusPending,
		Notes:            input.Notes,
	}

	return s.repo.CreateVerification(ctx, v)
}

// ReviewVerification processes an admin's decision on a verification.
// If this approval satisfies the global verification rules, the property is marked as Verified.
func (s *Service) ReviewVerification(ctx context.Context, verificationID string, adminID string, input UpdateVerificationInput) error {
	v, err := s.repo.GetVerificationByID(ctx, verificationID)
	if err != nil {
		return err
	}

	v.Status = input.Status
	v.AdminID = &adminID
	if input.Notes != nil {
		v.Notes = input.Notes
	}

	if err := s.repo.UpdateVerification(ctx, v); err != nil {
		return err
	}

	// If rejected, we don't automatically delist, we just leave it pending.
	// We only take action on the property if this was an approval.
	if input.Status == StatusApproved {
		return s.checkAndPromoteProperty(ctx, v.PropertyID)
	}

	return nil
}

// checkAndPromoteProperty evaluates all verifications for a property.
// BR: A property needs two approved verifications:
// 1. ai_photo
// 2. manual OR virtual_tour
func (s *Service) checkAndPromoteProperty(ctx context.Context, propertyID string) error {
	list, err := s.repo.GetVerificationsByProperty(ctx, propertyID)
	if err != nil {
		return err
	}

	hasPhoto := false
	hasManualOrTour := false

	for _, v := range list {
		if v.Status == StatusApproved {
			if v.VerificationType == TypeAIPhoto {
				hasPhoto = true
			}
			if v.VerificationType == TypeManual || v.VerificationType == TypeVirtualTour {
				hasManualOrTour = true
			}
		}
	}

	if hasPhoto && hasManualOrTour {
		// All conditions met! Promote the property to 'verified'.
		return s.propertyRepo.UpdateStatus(ctx, propertyID, property.StatusVerified)
	}

	return nil
}
