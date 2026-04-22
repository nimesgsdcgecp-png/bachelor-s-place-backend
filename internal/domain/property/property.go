package property

import (
	"errors"
	"time"

	"namenotdecidedyet/internal/pkg/apierror"
)

const (
	StatusDraft               = "draft"
	StatusPendingVerification = "pending_verification"
	StatusVerified            = "verified"
	StatusOccupied            = "occupied"
	StatusDelisted            = "delisted"
)

// Property represents a rentable unit or PG building.
type Property struct {
	ID            string     `json:"id"`
	OwnerID       string     `json:"owner_id"`
	Title         string     `json:"title"`
	Description   *string    `json:"description,omitempty"`
	PropertyType  string     `json:"property_type"`
	LocationLat   float64    `json:"location_lat"`
	LocationLng   float64    `json:"location_lng"`
	AddressText   *string    `json:"address_text,omitempty"`
	City          *string    `json:"city,omitempty"`
	Locality      *string    `json:"locality,omitempty"`
	RentAmount    *float64   `json:"rent_amount,omitempty"`
	DepositAmount *float64   `json:"deposit_amount,omitempty"`
	TotalCapacity *int       `json:"total_capacity,omitempty"`
	LifestyleTags []string   `json:"lifestyle_tags"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// CreatePropertyInput validates landlord input.
type CreatePropertyInput struct {
	Title         string   `json:"title"          validate:"required,min=5,max=100"`
	Description   *string  `json:"description"    validate:"omitempty,max=1000"`
	PropertyType  string   `json:"property_type"  validate:"required,oneof=room flat pg studio"`
	LocationLat   float64  `json:"location_lat"   validate:"required,latitude"`
	LocationLng   float64  `json:"location_lng"   validate:"required,longitude"`
	AddressText   *string  `json:"address_text"   validate:"omitempty,max=500"`
	City          *string  `json:"city"           validate:"omitempty,max=100"`
	Locality      *string  `json:"locality"       validate:"omitempty,max=100"`
	RentAmount    *float64 `json:"rent_amount"    validate:"omitempty,min=0"`
	DepositAmount *float64 `json:"deposit_amount" validate:"omitempty,min=0"`
	TotalCapacity *int     `json:"total_capacity" validate:"omitempty,min=1"`
	LifestyleTags []string `json:"lifestyle_tags" validate:"omitempty,max=10,dive,min=2,max=30"`
}

// SearchFilter represents dynamic query params for map search.
type SearchFilter struct {
	Lat      *float64
	Lng      *float64
	RadiusKm *float64
	City     *string
	Locality *string
	MinRent  *float64
	MaxRent  *float64
}

var (
	ErrKYCRequired      = errors.New("property: verified KYC required")
	ErrPropertyNotFound = errors.New("property: not found")
	ErrInvalidPGConfig  = errors.New("property: PG types cannot have a property-level rent_amount")
)

// ToAPIError maps domain errors to apierror.APIError.
func ToAPIError(err error) *apierror.APIError {
	switch {
	case errors.Is(err, ErrKYCRequired):
		return apierror.Forbidden("a verified KYC is required to list properties")
	case errors.Is(err, ErrPropertyNotFound):
		return apierror.NotFound("property not found")
	case errors.Is(err, ErrInvalidPGConfig):
		return apierror.BusinessRuleViolation("PG properties must have rent set at the room level, not property level")
	default:
		return apierror.Internal("an unexpected error occurred")
	}
}
