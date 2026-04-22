package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"namenotdecidedyet/internal/domain/property"
	"namenotdecidedyet/internal/domain/user"
	"namenotdecidedyet/internal/middleware"
	"namenotdecidedyet/internal/pkg/apierror"
	"namenotdecidedyet/internal/pkg/respond"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

type PropertyHandler struct {
	svc      *property.Service
	validate *validator.Validate
}

func NewPropertyHandler(svc *property.Service) *PropertyHandler {
	return &PropertyHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// CreateProperty handles POST /api/v1/properties
func (h *PropertyHandler) CreateProperty(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var input property.CreatePropertyInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apierror.ValidationError("invalid JSON body"))
		return
	}
	if err := h.validate.Struct(input); err != nil {
		respond.Error(w, apierror.ValidationError(err.Error()))
		return
	}

	id, err := h.svc.CreateProperty(r.Context(), userID, input)
	if err != nil {
		respond.Error(w, property.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]string{"id": id})
}

// GetProperty handles GET /api/v1/properties/{id}
func (h *PropertyHandler) GetProperty(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	p, err := h.svc.GetProperty(r.Context(), id)
	if err != nil {
		respond.Error(w, property.ToAPIError(err))
		return
	}

	respond.JSON(w, http.StatusOK, p)
}

// SearchProperties handles GET /api/v1/properties
func (h *PropertyHandler) SearchProperties(w http.ResponseWriter, r *http.Request) {
	var filter property.SearchFilter
	q := r.URL.Query()

	if lat := q.Get("lat"); lat != "" {
		if v, err := strconv.ParseFloat(lat, 64); err == nil {
			filter.Lat = &v
		}
	}
	if lng := q.Get("lng"); lng != "" {
		if v, err := strconv.ParseFloat(lng, 64); err == nil {
			filter.Lng = &v
		}
	}
	if radius := q.Get("radius_km"); radius != "" {
		if v, err := strconv.ParseFloat(radius, 64); err == nil {
			filter.RadiusKm = &v
		}
	}
	if city := q.Get("city"); city != "" {
		filter.City = &city
	}
	if locality := q.Get("locality"); locality != "" {
		filter.Locality = &locality
	}
	if minRent := q.Get("min_rent"); minRent != "" {
		if v, err := strconv.ParseFloat(minRent, 64); err == nil {
			filter.MinRent = &v
		}
	}
	if maxRent := q.Get("max_rent"); maxRent != "" {
		if v, err := strconv.ParseFloat(maxRent, 64); err == nil {
			filter.MaxRent = &v
		}
	}

	// Validate required map search params (if one coordinate is provided, need all three)
	if filter.Lat != nil || filter.Lng != nil || filter.RadiusKm != nil {
		if filter.Lat == nil || filter.Lng == nil || filter.RadiusKm == nil {
			respond.Error(w, apierror.ValidationError("lat, lng, and radius_km must all be provided together for map search"))
			return
		}
	}

	results, err := h.svc.SearchProperties(r.Context(), filter)
	if err != nil {
		respond.Error(w, property.ToAPIError(err))
		return
	}

	if results == nil {
		results = []property.Property{} // Return [] instead of null
	}

	respond.JSON(w, http.StatusOK, results)
}

// Routes returns the property routes.
func (h *PropertyHandler) Routes() chi.Router {
	r := chi.NewRouter()
	
	// Anyone authenticated can view/search
	r.Get("/", h.SearchProperties)
	r.Get("/{id}", h.GetProperty)

	// Only landlords can create
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole(user.RoleLandlord))
		r.Post("/", h.CreateProperty)
	})

	return r
}
