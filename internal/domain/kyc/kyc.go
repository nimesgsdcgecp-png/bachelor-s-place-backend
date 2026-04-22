package kyc

import (
	"errors"
	"time"

	"namenotdecidedyet/internal/pkg/apierror"
)

const (
	StatusPending  = "pending"
	StatusVerified = "verified"
	StatusRejected = "rejected"
)

// LandlordKYC represents a KYC submission.
// PII fields (Aadhaar, PAN) are never exposed via the API responses.
type LandlordKYC struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	AadhaarEncrypted *string    `json:"-"`
	PANEncrypted     *string    `json:"-"`
	AadhaarVerified  bool       `json:"aadhaar_verified"`
	PANVerified      bool       `json:"pan_verified"`
	Status           string     `json:"status"`
	SubmittedAt      *time.Time `json:"submitted_at,omitempty"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// SubmitKYCInput is the validated request body for POST /api/v1/kyc.
type SubmitKYCInput struct {
	Aadhaar string `json:"aadhaar" validate:"required,len=12,numeric"`
	PAN     string `json:"pan"     validate:"required,len=10,alphanum"`
}

// ReviewKYCInput is the request body for PUT /api/v1/admin/kyc/{id}/review.
type ReviewKYCInput struct {
	Status string `json:"status" validate:"required,oneof=verified rejected"`
}

var (
	ErrKYCAlreadyExists = errors.New("kyc: submission already exists")
	ErrKYCNotFound      = errors.New("kyc: submission not found")
	ErrInvalidStatus    = errors.New("kyc: invalid status transition")
	ErrOnlyLandlords    = errors.New("kyc: only landlords can submit KYC")
)

// ToAPIError maps domain errors to apierror types for handler use.
func ToAPIError(err error) *apierror.APIError {
	switch {
	case errors.Is(err, ErrKYCAlreadyExists):
		return apierror.Conflict("a KYC submission already exists for this user")
	case errors.Is(err, ErrKYCNotFound):
		return apierror.NotFound("KYC submission not found")
	case errors.Is(err, ErrInvalidStatus):
		return apierror.BusinessRuleViolation("invalid KYC status transition")
	case errors.Is(err, ErrOnlyLandlords):
		return apierror.Forbidden("only users with the landlord role can submit KYC")
	default:
		return apierror.Internal("an unexpected error occurred")
	}
}
