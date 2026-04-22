package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"namenotdecidedyet/internal/domain/squad"
	"namenotdecidedyet/internal/middleware"
	"namenotdecidedyet/internal/pkg/apierror"
	"namenotdecidedyet/internal/pkg/respond"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

type SquadHandler struct {
	svc      *squad.Service
	validate *validator.Validate
}

func NewSquadHandler(svc *squad.Service) *SquadHandler {
	return &SquadHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

func (h *SquadHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Route("/squad-lookups", func(r chi.Router) {
		r.Post("/", h.RegisterLookup)
		r.Get("/matches", h.GetMatches)
	})

	r.Route("/squads", func(r chi.Router) {
		r.Post("/", h.CreateSquad)
		r.Get("/{id}", h.GetSquad)
		r.Post("/{id}/invite", h.InviteMember)
		r.Put("/{id}/members/me", h.RespondToInvite)
		r.Post("/{id}/proposals", h.ProposeProperty)
	})
	r.Put("/squads/proposals/{proposalId}", h.ResolveProposal)

	return r
}

func (h *SquadHandler) RegisterLookup(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var input struct {
		PropertyID         *string `json:"property_id"`
		LocalityPreference string  `json:"locality_preference"`
		BudgetMin          float64 `json:"budget_min" validate:"required,gt=0"`
		BudgetMax          float64 `json:"budget_max" validate:"required,gtfield=BudgetMin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body"))
		return
	}

	lookup := &squad.SquadLookup{
		UserID:             userID,
		PropertyID:         input.PropertyID,
		LocalityPreference: input.LocalityPreference,
		BudgetMin:          &input.BudgetMin,
		BudgetMax:          &input.BudgetMax,
	}

	id, err := h.svc.RegisterLookup(r.Context(), lookup)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *SquadHandler) GetMatches(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	matches, err := h.svc.GetMatches(r.Context(), userID, page, perPage)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, matches)
}

func (h *SquadHandler) CreateSquad(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var input struct {
		Name       string  `json:"name" validate:"required,min=3"`
		PropertyID *string `json:"property_id"`
		RoomID     *string `json:"room_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body"))
		return
	}

	id, err := h.svc.CreateSquad(r.Context(), input.Name, userID, input.PropertyID, input.RoomID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *SquadHandler) GetSquad(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	details, err := h.svc.GetSquadDetails(r.Context(), userID, id)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, details)
}

func (h *SquadHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
	senderID := middleware.UserIDFromContext(r.Context())
	squadID := chi.URLParam(r, "id")
	var input struct {
		UserID string `json:"user_id" validate:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body"))
		return
	}

	err := h.svc.InviteMember(r.Context(), senderID, squadID, input.UserID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{"message": "invitation sent"})
}

func (h *SquadHandler) RespondToInvite(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	squadID := chi.URLParam(r, "id")
	var input struct {
		Action string `json:"action" validate:"required,oneof=accept reject"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body"))
		return
	}

	err := h.svc.RespondToInvite(r.Context(), userID, squadID, input.Action == "accept")
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{"message": "response processed"})
}

func (h *SquadHandler) ProposeProperty(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	squadID := chi.URLParam(r, "id")
	var input struct {
		PropertyID string  `json:"property_id" validate:"required"`
		RoomID     *string `json:"room_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body"))
		return
	}

	id, err := h.svc.ProposeProperty(r.Context(), userID, squadID, input.PropertyID, input.RoomID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *SquadHandler) ResolveProposal(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	proposalID := chi.URLParam(r, "proposalId")
	var input struct {
		Action string `json:"action" validate:"required,oneof=accept reject"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body"))
		return
	}

	err := h.svc.ResolveProposal(r.Context(), userID, proposalID, input.Action == "accept")
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{"message": "proposal resolved"})
}
