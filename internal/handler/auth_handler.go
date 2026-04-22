package handler

import (
	"encoding/json"
	"net/http"

	"namenotdecidedyet/internal/domain/user"
	"namenotdecidedyet/internal/pkg/apierror"
	"namenotdecidedyet/internal/pkg/respond"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// AuthHandler handles authentication HTTP requests.
// Handlers are thin: decode → validate → call service → respond.
// No business logic lives here.
type AuthHandler struct {
	svc      *user.Service
	validate *validator.Validate
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(svc *user.Service) *AuthHandler {
	return &AuthHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// Register handles POST /api/v1/auth/register
// Creates a new tenant or landlord account.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input user.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.ValidationError("invalid JSON body"))
		return
	}
	if err := h.validate.Struct(input); err != nil {
		respond.Error(w, apierror.ValidationError(err.Error()))
		return
	}

	result, err := h.svc.Register(r.Context(), input)
	if err != nil {
		respond.Error(w, user.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusCreated, result)
}

// Login handles POST /api/v1/auth/login
// Validates credentials and returns a JWT access + refresh token pair.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input user.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.ValidationError("invalid JSON body"))
		return
	}
	if err := h.validate.Struct(input); err != nil {
		respond.Error(w, apierror.ValidationError(err.Error()))
		return
	}

	result, err := h.svc.Login(r.Context(), input)
	if err != nil {
		respond.Error(w, user.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusOK, result)
}

// Refresh handles POST /api/v1/auth/refresh
// Accepts a refresh token and issues a new access + refresh token pair.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var input user.RefreshInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.ValidationError("invalid JSON body"))
		return
	}
	if err := h.validate.Struct(input); err != nil {
		respond.Error(w, apierror.ValidationError(err.Error()))
		return
	}

	result, err := h.svc.RefreshToken(r.Context(), input)
	if err != nil {
		respond.Error(w, user.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusOK, result)
}

// Routes returns a chi.Router with all auth routes mounted.
func (h *AuthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/refresh", h.Refresh)
	return r
}

