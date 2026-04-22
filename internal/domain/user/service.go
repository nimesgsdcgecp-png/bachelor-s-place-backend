package user

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenDuration  = 24 * time.Hour
	refreshTokenDuration = 7 * 24 * time.Hour
	bcryptCost           = 12
)

// Repository defines the database operations required by the user Service.
// Implemented by *repository.UserRepo — the service depends on this interface,
// not the concrete type, keeping the domain layer decoupled from the DB layer.
type Repository interface {
	CreateUser(ctx context.Context, u *User) (string, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) error
}

// Service handles all user and authentication business logic.
type Service struct {
	repo      Repository
	jwtSecret []byte
}

// NewService creates a new user Service.
func NewService(repo Repository, jwtSecret string) *Service {
	return &Service{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
	}
}

// Register creates a new tenant or landlord account.
// BR-02: admin registration via this endpoint is forbidden.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*RegisterResponse, error) {
	if input.Role == RoleAdmin {
		return nil, ErrInvalidRole
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("user service: failed to hash password: %w", err)
	}

	u := &User{
		Name:                input.Name,
		Email:               input.Email,
		PasswordHash:        string(hash),
		Role:                input.Role,
		LifestyleTags:       []string{},
		PreferredLocalities: []string{},
	}

	id, err := s.repo.CreateUser(ctx, u)
	if err != nil {
		return nil, err // ErrEmailAlreadyExists bubbles up as-is
	}

	return &RegisterResponse{UserID: id, Role: input.Role}, nil
}

// Login validates credentials and returns an access + refresh token pair.
// Email enumeration is avoided: ErrUserNotFound is mapped to ErrInvalidCredentials.
func (s *Service) Login(ctx context.Context, input LoginInput) (*AuthResponse, error) {
	u, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		// Do NOT distinguish "no user" from "wrong password" — prevents email enumeration
		return nil, ErrInvalidCredentials
	}

	if !u.IsActive {
		return nil, ErrAccountInactive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.generateTokenPair(u.ID, u.Role)
}

// RefreshToken validates a refresh JWT and issues a new access + refresh token pair.
func (s *Service) RefreshToken(_ context.Context, input RefreshInput) (*AuthResponse, error) {
	claims, err := s.parseToken(input.RefreshToken, "refresh")
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	return s.generateTokenPair(claims.userID, claims.role)
}

// GetUserByID is exposed for use by other domain services and handlers.
func (s *Service) GetUserByID(ctx context.Context, id string) (*User, error) {
	return s.repo.GetUserByID(ctx, id)
}

// UpdateProfile updates the user's lifestyle profile and triggers background embedding generation.
func (s *Service) UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) error {
	// Let repo handle setting pending_embeddings = TRUE
	return s.repo.UpdateProfile(ctx, userID, input)
}

// --- private helpers --------------------------------------------------------

func (s *Service) generateTokenPair(userID, role string) (*AuthResponse, error) {
	accessToken, err := s.signToken(userID, role, "access", accessTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("user service: failed to sign access token: %w", err)
	}
	refreshToken, err := s.signToken(userID, role, "refresh", refreshTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("user service: failed to sign refresh token: %w", err)
	}
	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(accessTokenDuration.Seconds()),
		Role:         role,
	}, nil
}

func (s *Service) signToken(userID, role, tokenType string, duration time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"user_id":    userID,
		"role":       role,
		"token_type": tokenType, // "access" | "refresh" — prevents token misuse
		"exp":        time.Now().Add(duration).Unix(),
		"iat":        time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

type parsedClaims struct {
	userID string
	role   string
}

func (s *Service) parseToken(tokenStr, expectedType string) (*parsedClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid or expired token")
	}

	mc, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}
	if tt, _ := mc["token_type"].(string); tt != expectedType {
		return nil, fmt.Errorf("wrong token type: expected %s", expectedType)
	}

	return &parsedClaims{
		userID: mc["user_id"].(string),
		role:   mc["role"].(string),
	}, nil
}
