package handler

import (
	"encoding/json"
	"net/http"

	"namenotdecidedyet/internal/domain/kyc"
	"namenotdecidedyet/internal/domain/user"
	"namenotdecidedyet/internal/middleware"
	"namenotdecidedyet/internal/pkg/apierror"
	"namenotdecidedyet/internal/pkg/respond"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

type KYCHandler struct {
	svc      *kyc.Service
	validate *validator.Validate
}

func NewKYCHandler(svc *kyc.Service) *KYCHandler {
	return &KYCHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// SubmitKYC handles POST /api/v1/kyc
func (h *KYCHandler) SubmitKYC(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var input kyc.SubmitKYCInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.ValidationError("invalid JSON body"))
		return
	}
	if err := h.validate.Struct(input); err != nil {
		respond.Error(w, apierror.ValidationError(err.Error()))
		return
	}

	if err := h.svc.SubmitKYC(r.Context(), userID, input); err != nil {
		respond.Error(w, kyc.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]string{"message": "KYC submitted successfully"})
}

// GetMyStatus handles GET /api/v1/kyc/me
func (h *KYCHandler) GetMyStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	status, err := h.svc.GetMyStatus(r.Context(), userID)
	if err != nil {
		respond.Error(w, kyc.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusOK, status)
}

// AdminListPending handles GET /api/v1/admin/kyc/pending
func (h *KYCHandler) AdminListPending(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListPending(r.Context())
	if err != nil {
		respond.Error(w, kyc.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusOK, list)
}

// AdminReviewKYC handles PUT /api/v1/admin/kyc/{id}/review
func (h *KYCHandler) AdminReviewKYC(w http.ResponseWriter, r *http.Request) {
	kycID := chi.URLParam(r, "id")

	var input kyc.ReviewKYCInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.ValidationError("invalid JSON body"))
		return
	}
	if err := h.validate.Struct(input); err != nil {
		respond.Error(w, apierror.ValidationError(err.Error()))
		return
	}

	if err := h.svc.ReviewKYC(r.Context(), kycID, input); err != nil {
		respond.Error(w, kyc.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{"message": "KYC status updated"})
}

// Routes returns the regular user facing KYC routes.
func (h *KYCHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequireRole(user.RoleLandlord))
	r.Post("/", h.SubmitKYC)
	r.Get("/me", h.GetMyStatus)
	return r
}

// AdminRoutes returns the admin facing KYC routes.
func (h *KYCHandler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequireRole(user.RoleAdmin))
	r.Get("/pending", h.AdminListPending)
	r.Put("/{id}/review", h.AdminReviewKYC)
	return r
}
