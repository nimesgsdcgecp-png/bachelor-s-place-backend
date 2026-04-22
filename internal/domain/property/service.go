package property

import (
	"context"

	"namenotdecidedyet/internal/domain/kyc"
	"namenotdecidedyet/internal/domain/user"
)

// Repository manages database interaction for properties.
type Repository interface {
	CreateProperty(ctx context.Context, p *Property) (string, error)
	GetPropertyByID(ctx context.Context, id string) (*Property, error)
	SearchProperties(ctx context.Context, filter SearchFilter) ([]Property, error)
	UpdateStatus(ctx context.Context, id string, status string) error
}

// Service handles property business logic.
type Service struct {
	repo     Repository
	kycRepo  kyc.Repository
	userRepo user.Repository
}

// NewService creates a new Property Service.
func NewService(repo Repository, kycRepo kyc.Repository, userRepo user.Repository) *Service {
	return &Service{
		repo:     repo,
		kycRepo:  kycRepo,
		userRepo: userRepo,
	}
}

// CreateProperty ensures the landlord has verified KYC before saving the listing.
func (s *Service) CreateProperty(ctx context.Context, ownerID string, input CreatePropertyInput) (string, error) {
	// BR-03: Landlord must have verified KYC to list
	k, err := s.kycRepo.GetKYCByUserID(ctx, ownerID)
	if err != nil {
		if err == kyc.ErrKYCNotFound {
			return "", ErrKYCRequired
		}
		return "", err
	}
	if k.Status != kyc.StatusVerified {
		return "", ErrKYCRequired
	}

	// BR: PG types cannot have a rent amount at the property level.
	if input.PropertyType == "pg" && input.RentAmount != nil {
		return "", ErrInvalidPGConfig
	}

	if input.LifestyleTags == nil {
		input.LifestyleTags = []string{}
	}

	p := &Property{
		OwnerID:       ownerID,
		Title:         input.Title,
		Description:   input.Description,
		PropertyType:  input.PropertyType,
		LocationLat:   input.LocationLat,
		LocationLng:   input.LocationLng,
		AddressText:   input.AddressText,
		City:          input.City,
		Locality:      input.Locality,
		RentAmount:    input.RentAmount,
		DepositAmount: input.DepositAmount,
		TotalCapacity: input.TotalCapacity,
		LifestyleTags: input.LifestyleTags,
		Status:        StatusDraft, // Starts as draft until explicitly published or verified
	}

	return s.repo.CreateProperty(ctx, p)
}

// GetProperty retrieves a single property by ID.
func (s *Service) GetProperty(ctx context.Context, id string) (*Property, error) {
	return s.repo.GetPropertyByID(ctx, id)
}

// SearchProperties performs a dynamic map-based search.
func (s *Service) SearchProperties(ctx context.Context, filter SearchFilter) ([]Property, error) {
	return s.repo.SearchProperties(ctx, filter)
}
