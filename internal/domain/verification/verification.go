package verification

import (
	"errors"
	"time"

	"namenotdecidedyet/internal/pkg/apierror"
)

const (
	TypeAIPhoto     = "ai_photo"
	TypeManual      = "manual"
	TypeVirtualTour = "virtual_tour"
	TypePhysical    = "physical"

	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

// Verification represents an admin review of a property listing.
type Verification struct {
	ID               string     `json:"id"`
	PropertyID       string     `json:"property_id"`
	AdminID          *string    `json:"admin_id,omitempty"`
	VerificationType string     `json:"verification_type"`
	Status           string     `json:"status"`
	Notes            *string    `json:"notes,omitempty"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// CreateVerificationInput is used by admins to initiate a verification process.
type CreateVerificationInput struct {
	VerificationType string  `json:"verification_type" validate:"required,oneof=ai_photo manual virtual_tour physical"`
	Notes            *string `json:"notes" validate:"omitempty,max=1000"`
}

// UpdateVerificationInput is used by admins to approve or reject.
type UpdateVerificationInput struct {
	Status string  `json:"status" validate:"required,oneof=approved rejected"`
	Notes  *string `json:"notes" validate:"omitempty,max=1000"`
}

var (
	ErrVerificationNotFound = errors.New("verification: not found")
	ErrPropertyAlreadyVerified = errors.New("verification: property is already verified")
)

// ToAPIError maps domain errors to standard API responses.
func ToAPIError(err error) *apierror.APIError {
	switch {
	case errors.Is(err, ErrVerificationNotFound):
		return apierror.NotFound("verification record not found")
	case errors.Is(err, ErrPropertyAlreadyVerified):
		return apierror.Conflict("this property is already fully verified")
	default:
		return apierror.Internal("an unexpected error occurred")
	}
}
