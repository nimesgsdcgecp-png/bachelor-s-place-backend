package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"namenotdecidedyet/internal/middleware"
	"namenotdecidedyet/internal/pkg/apierror"
	"namenotdecidedyet/internal/pkg/respond"
	"namenotdecidedyet/internal/repository"

	"github.com/go-chi/chi/v5"
)

type MessageHandler struct {
	repo      *repository.MessageRepo
	jwtSecret string
}

func NewMessageHandler(repo *repository.MessageRepo, jwtSecret string) *MessageHandler {
	return &MessageHandler{repo: repo, jwtSecret: jwtSecret}
}

// Routes mounts under /squads/{squadId}/messages
func (h *MessageHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.jwtSecret))

	r.Get("/", h.GetMessages)
	r.Post("/", h.SendMessage)
	r.Put("/read", h.MarkRead)

	return r
}

// GetMessages handles GET /api/v1/squads/{squadId}/messages
// Cursor-based pagination — pass ?cursor=<sent_at ISO timestamp> for older messages.
func (h *MessageHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	squadID := chi.URLParam(r, "squadId")

	// Guard: only accepted members can read chat (FR-4.5)
	ok, err := h.repo.IsMember(r.Context(), squadID, userID)
	if err != nil || !ok {
		respond.Error(w, apierror.Forbidden("you are not an accepted member of this squad"))
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit := 30
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}

	messages, err := h.repo.GetMessages(r.Context(), squadID, cursor, limit)
	if err != nil {
		respond.Error(w, apierror.Internal(err.Error()))
		return
	}

	if messages == nil {
		messages = []*repository.Message{}
	}

	// Provide next_cursor for the client to fetch older messages
	var nextCursor string
	if len(messages) == limit {
		nextCursor = messages[len(messages)-1].SentAt.Format("2006-01-02T15:04:05.999999Z07:00")
	}

	respond.JSON(w, http.StatusOK, map[string]interface{}{
		"messages":    messages,
		"count":       len(messages),
		"next_cursor": nextCursor,
	})
}

// SendMessage handles POST /api/v1/squads/{squadId}/messages
func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	squadID := chi.URLParam(r, "squadId")

	// Guard: only accepted members can send messages
	ok, err := h.repo.IsMember(r.Context(), squadID, userID)
	if err != nil || !ok {
		respond.Error(w, apierror.Forbidden("you are not an accepted member of this squad"))
		return
	}

	var input struct {
		Content     string `json:"content"`
		ContentType string `json:"content_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Content == "" {
		respond.Error(w, apierror.ValidationError("content is required"))
		return
	}

	contentType := input.ContentType
	if contentType == "" {
		contentType = "text"
	}

	msg, err := h.repo.CreateMessage(r.Context(), squadID, userID, input.Content, contentType)
	if err != nil {
		respond.Error(w, apierror.Internal(err.Error()))
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]interface{}{
		"message": msg,
	})
}

// MarkRead handles PUT /api/v1/squads/{squadId}/messages/read
// Marks all unread messages in the squad as read by the current user.
func (h *MessageHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	squadID := chi.URLParam(r, "squadId")

	// Guard: only accepted members can mark read
	ok, err := h.repo.IsMember(r.Context(), squadID, userID)
	if err != nil || !ok {
		respond.Error(w, apierror.Forbidden("you are not an accepted member of this squad"))
		return
	}

	count, err := h.repo.MarkRead(r.Context(), squadID, userID)
	if err != nil {
		respond.Error(w, apierror.Internal(err.Error()))
		return
	}

	respond.JSON(w, http.StatusOK, map[string]interface{}{
		"marked_read": count,
	})
}
