package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"namenotdecidedyet/internal/domain/transaction"
	"namenotdecidedyet/internal/middleware"
	"namenotdecidedyet/internal/pkg/apierror"
	"namenotdecidedyet/internal/pkg/respond"

	"github.com/go-chi/chi/v5"
)

type TransactionHandler struct {
	service   *transaction.Service
	jwtSecret string
}

func NewTransactionHandler(service *transaction.Service, jwtSecret string) *TransactionHandler {
	return &TransactionHandler{service: service, jwtSecret: jwtSecret}
}

func (h *TransactionHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(h.jwtSecret))
		r.Post("/squads/{squadId}/pay-token", h.PayToken)
		r.Post("/squads/{squadId}/move-in", h.MoveIn)
		r.Get("/history", h.GetHistory)
	})

	// Public webhook endpoint — called by Razorpay, no JWT
	// IMPORTANT: Must read raw body for signature verification
	r.Post("/webhook", h.HandleWebhook)

	return r
}

// PayToken handles POST /api/v1/payments/squads/{squadId}/pay-token
// Initiates a Razorpay order and returns the order details for frontend checkout.
func (h *TransactionHandler) PayToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	squadID := chi.URLParam(r, "squadId")

	tx, order, err := h.service.InitiateTokenPayment(r.Context(), userID, squadID)
	if err != nil {
		respond.Error(w, apierror.Internal(err.Error()))
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]interface{}{
		"transaction":   tx,
		"gateway_order": order,
		"checkout_info": map[string]interface{}{
			"key_id":   order.KeyID,
			"order_id": order.ID,
			"amount":   order.Amount,
			"currency": order.Currency,
			"name":     "BachelorPad",
			"description": "Property Token Payment",
		},
	})
}

// HandleWebhook handles POST /api/v1/payments/webhook
// Called by Razorpay — reads raw body for HMAC-SHA256 signature validation.
// Per Razorpay docs: always return 200 even on internal errors to prevent retries.
func (h *TransactionHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Read raw body FIRST — needed for signature verification
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		respond.JSON(w, http.StatusOK, map[string]string{"status": "error", "message": "could not read body"})
		return
	}

	// Razorpay sends signature in this header
	signature := r.Header.Get("X-Razorpay-Signature")

	if err := h.service.ProcessWebhook(r.Context(), rawBody, signature); err != nil {
		// Log but always return 200 to prevent Razorpay infinite retries
		respond.JSON(w, http.StatusOK, map[string]string{"status": "error", "message": err.Error()})
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetHistory handles GET /api/v1/payments/history
// Returns the authenticated user's transaction history.
func (h *TransactionHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	txs, err := h.service.GetTransactionHistory(r.Context(), userID)
	if err != nil {
		respond.Error(w, apierror.Internal(err.Error()))
		return
	}

	if txs == nil {
		txs = []*transaction.Transaction{} // return empty array not null
	}

	respond.JSON(w, http.StatusOK, map[string]interface{}{
		"transactions": txs,
		"count":        len(txs),
	})
}

// MoveIn handles POST /api/v1/payments/squads/{squadId}/move-in
// Confirms the squad has moved in: squad → moved_in, property → occupied, success_fee recorded.
func (h *TransactionHandler) MoveIn(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	squadID := chi.URLParam(r, "squadId")

	if err := h.service.ConfirmMoveIn(r.Context(), userID, squadID); err != nil {
		respond.Error(w, apierror.Internal(err.Error()))
		return
	}

	respond.JSON(w, http.StatusOK, map[string]interface{}{
		"message":  "move-in confirmed successfully",
		"squad_id": squadID,
		"status":   "moved_in",
	})
}

// parseWebhookBody is a helper for test-only dev webhook simulation
func parseWebhookBody(body []byte) (gatewayRefID, signature string, err error) {
	var input struct {
		GatewayRefID string `json:"gateway_reference_id"`
		Signature    string `json:"signature"`
	}
	if err = json.Unmarshal(body, &input); err != nil {
		return "", "", err
	}
	return input.GatewayRefID, input.Signature, nil
}
