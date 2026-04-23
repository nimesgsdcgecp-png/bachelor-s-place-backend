package handler

import (
	"net/http"
	"strconv"

	"namenotdecidedyet/internal/middleware"
	"namenotdecidedyet/internal/pkg/apierror"
	"namenotdecidedyet/internal/pkg/respond"
	"namenotdecidedyet/internal/repository"

	"github.com/go-chi/chi/v5"
)

type NotificationHandler struct {
	repo      *repository.NotificationRepo
	jwtSecret string
}

func NewNotificationHandler(repo *repository.NotificationRepo, jwtSecret string) *NotificationHandler {
	return &NotificationHandler{repo: repo, jwtSecret: jwtSecret}
}

func (h *NotificationHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.jwtSecret))

	r.Get("/", h.GetNotifications)
	r.Put("/read-all", h.MarkAllRead)      // must be before /{id}/read to avoid routing conflict
	r.Put("/{id}/read", h.MarkOneRead)

	return r
}

// GetNotifications handles GET /api/v1/notifications
// Returns paginated notification feed — unread first, then newest first.
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	notifications, total, err := h.repo.GetForUser(r.Context(), userID, page, perPage)
	if err != nil {
		respond.Error(w, apierror.Internal(err.Error()))
		return
	}

	if notifications == nil {
		notifications = []*repository.Notification{}
	}

	respond.JSON(w, http.StatusOK, map[string]interface{}{
		"notifications": notifications,
		"total":         total,
		"page":          page,
		"per_page":      perPage,
	})
}

// MarkOneRead handles PUT /api/v1/notifications/{id}/read
func (h *NotificationHandler) MarkOneRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	notifID := chi.URLParam(r, "id")

	if err := h.repo.MarkOneRead(r.Context(), notifID, userID); err != nil {
		respond.Error(w, apierror.NotFound("notification not found"))
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{"message": "marked as read"})
}

// MarkAllRead handles PUT /api/v1/notifications/read-all
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	count, err := h.repo.MarkAllRead(r.Context(), userID)
	if err != nil {
		respond.Error(w, apierror.Internal(err.Error()))
		return
	}

	respond.JSON(w, http.StatusOK, map[string]interface{}{
		"marked_read": count,
	})
}
