package squad

import (
	"context"
	"errors"
	"time"
)

// Domain Errors
var (
	ErrSquadNotFound      = errors.New("squad not found")
	ErrSquadFull          = errors.New("squad has reached maximum capacity")
	ErrAlreadyInSquad     = errors.New("user is already a member of this squad")
	ErrNotSquadLeader     = errors.New("only the squad leader can perform this action")
	ErrLookupNotFound     = errors.New("active squad lookup not found")
	ErrProposalNotFound   = errors.New("property proposal not found")
	ErrUnauthorizedAction = errors.New("unauthorized action for this squad")
)

// Squad Status
type Status string

const (
	StatusBrowsing Status = "browsing" // No property selected yet
	StatusForming  Status = "forming"  // Property identified; members finalizing
	StatusLocked   Status = "locked"   // Token paid; property reserved
	StatusMovedIn  Status = "moved_in" // Move-in confirmed
	StatusDisbanded Status = "disbanded"
)

// Member Status & Role
type MemberStatus string
type MemberRole string

const (
	MemberStatusInvited  MemberStatus = "invited"
	MemberStatusAccepted MemberStatus = "accepted"
	MemberStatusRejected MemberStatus = "rejected"
	MemberStatusLeft     MemberStatus = "left"

	MemberRoleLeader MemberRole = "leader"
	MemberRoleMember MemberRole = "member"
)

// Squad represents a group of 2-5 bachelors renting together.
type Squad struct {
	ID                   string     `json:"id"`
	PropertyID           *string    `json:"property_id,omitempty"` // NULL when browsing
	RoomID               *string    `json:"room_id,omitempty"`
	Name                 string     `json:"name"`
	Status               Status     `json:"status"`
	MaxSize              int        `json:"max_size"`
	CurrentMemberCount   int        `json:"current_member_count"`
	CreatedBy            string     `json:"created_by"`
	TotalDepositCollected float64    `json:"total_deposit_collected"`
	TokenPaidAt          *time.Time `json:"token_paid_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// SquadMember represents a user's membership in a squad.
type SquadMember struct {
	ID          string       `json:"id"`
	SquadID     string       `json:"squad_id"`
	UserID      string       `json:"user_id"`
	UserName    string       `json:"user_name,omitempty"` // For UI lists
	Role        MemberRole   `json:"role"`
	Status      MemberStatus `json:"status"`
	ShareAmount *float64     `json:"share_amount"`
	JoinedAt    *time.Time   `json:"joined_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}

// SquadLookup represents a tenant's intent to find a squad.
type SquadLookup struct {
	ID                 string    `json:"id"`
	UserID             string    `json:"user_id"`
	PropertyID         *string   `json:"property_id,omitempty"` // NULL = squad-first flow
	LocalityPreference string    `json:"locality_preference,omitempty"`
	BudgetMin          *float64  `json:"budget_min"`
	BudgetMax          *float64  `json:"budget_max"`
	Status             string    `json:"status"` // active | matched | inactive
	CreatedAt          time.Time `json:"created_at"`
	ExpiresAt          time.Time `json:"expires_at"`
}

// MatchResult represents a compatible user found via pgvector similarity.
type MatchResult struct {
	UserID            string   `json:"user_id"`
	Name              string   `json:"name"`
	LifestyleTags     []string `json:"lifestyle_tags"`
	Bio               string   `json:"bio"`
	CompatibilityScore float64  `json:"compatibility_score"`
}

// Repository interface for squad data access.
type Repository interface {
	// Lookups
	CreateLookup(ctx context.Context, lookup *SquadLookup) (string, error)
	GetActiveLookup(ctx context.Context, userID string) (*SquadLookup, error)
	DeleteLookup(ctx context.Context, userID string) error
	
	// Matching (The pgvector core)
	FindMatches(ctx context.Context, userID string, limit int, offset int) ([]MatchResult, error)

	// Squads
	CreateSquad(ctx context.Context, squad *Squad, leaderID string) (string, error)
	GetSquadByID(ctx context.Context, id string) (*Squad, error)
	GetMembers(ctx context.Context, squadID string) ([]SquadMember, error)
	
	// Invites & Membership
	AddMember(ctx context.Context, squadID, userID string, role MemberRole, status MemberStatus) error
	UpdateMemberStatus(ctx context.Context, squadID, userID string, status MemberStatus) error
	RemoveMember(ctx context.Context, squadID, userID string) error
	
	// Proposals
	CreateProposal(ctx context.Context, squadID, userID, propertyID string, roomID *string) (string, error)
	GetProposals(ctx context.Context, squadID string) ([]map[string]interface{}, error)
	ResolveProposal(ctx context.Context, proposalID string, status string) error
}
