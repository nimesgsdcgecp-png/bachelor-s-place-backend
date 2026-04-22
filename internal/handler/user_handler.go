package handler

import (
	"encoding/json"
	"net/http"

	"namenotdecidedyet/internal/domain/user"
	"namenotdecidedyet/internal/middleware"
	"namenotdecidedyet/internal/pkg/apierror"
	"namenotdecidedyet/internal/pkg/respond"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// UserHandler handles user profile HTTP requests.
type UserHandler struct {
	svc      *user.Service
	validate *validator.Validate
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(svc *user.Service) *UserHandler {
	return &UserHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// GetProfile handles GET /api/v1/users/me
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	
	u, err := h.svc.GetUserByID(r.Context(), userID)
	if err != nil {
		respond.Error(w, user.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusOK, u)
}

// UpdateProfile handles PUT /api/v1/users/me/profile
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var input user.UpdateProfileInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.ValidationError("invalid JSON body"))
		return
	}
	if err := h.validate.Struct(input); err != nil {
		respond.Error(w, apierror.ValidationError(err.Error()))
		return
	}

	if err := h.svc.UpdateProfile(r.Context(), userID, input); err != nil {
		respond.Error(w, user.ToAPIError(err))
		return
	}

	// Fetch updated profile to return
	u, err := h.svc.GetUserByID(r.Context(), userID)
	if err != nil {
		respond.Error(w, user.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusOK, u)
}

// Routes returns a chi.Router with all user profile routes mounted.
func (h *UserHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/me", h.GetProfile)
	r.Put("/me/profile", h.UpdateProfile)
	return r
}
