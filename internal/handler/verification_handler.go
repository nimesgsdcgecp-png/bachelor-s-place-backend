package handler

import (
	"encoding/json"
	"net/http"

	"namenotdecidedyet/internal/domain/user"
	"namenotdecidedyet/internal/domain/verification"
	"namenotdecidedyet/internal/middleware"
	"namenotdecidedyet/internal/pkg/apierror"
	"namenotdecidedyet/internal/pkg/respond"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

type VerificationHandler struct {
	svc      *verification.Service
	validate *validator.Validate
}

func NewVerificationHandler(svc *verification.Service) *VerificationHandler {
	return &VerificationHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// InitiateVerification handles POST /api/v1/admin/properties/{propertyID}/verifications
func (h *VerificationHandler) InitiateVerification(w http.ResponseWriter, r *http.Request) {
	propertyID := chi.URLParam(r, "propertyID")
	adminID := middleware.UserIDFromContext(r.Context())

	var input verification.CreateVerificationInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.ValidationError("invalid JSON body"))
		return
	}
	if err := h.validate.Struct(input); err != nil {
		respond.Error(w, apierror.ValidationError(err.Error()))
		return
	}

	id, err := h.svc.InitiateVerification(r.Context(), propertyID, adminID, input)
	if err != nil {
		respond.Error(w, verification.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]string{"id": id})
}

// ReviewVerification handles PUT /api/v1/admin/verifications/{id}
func (h *VerificationHandler) ReviewVerification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	adminID := middleware.UserIDFromContext(r.Context())

	var input verification.UpdateVerificationInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.ValidationError("invalid JSON body"))
		return
	}
	if err := h.validate.Struct(input); err != nil {
		respond.Error(w, apierror.ValidationError(err.Error()))
		return
	}

	if err := h.svc.ReviewVerification(r.Context(), id, adminID, input); err != nil {
		respond.Error(w, verification.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{"message": "verification processed"})
}

// AdminRoutes returns the admin verification routes.
func (h *VerificationHandler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequireRole(user.RoleAdmin))

	// Note: We'll mount this at /api/v1/admin
	// so the actual path becomes /api/v1/admin/verifications/...
	r.Put("/verifications/{id}", h.ReviewVerification)
	r.Post("/properties/{propertyID}/verifications", h.InitiateVerification)

	return r
}
