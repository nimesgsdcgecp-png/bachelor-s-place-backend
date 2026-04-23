package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
)

const razorpayOrdersURL = "https://api.razorpay.com/v1/orders"

// RazorpayGateway implements Gateway using the Razorpay REST API.
// No external SDK — uses raw HTTP + standard library only.
type RazorpayGateway struct {
	keyID         string
	keySecret     string
	webhookSecret string
	httpClient    *http.Client
}

// NewRazorpayGateway creates a production-ready Razorpay gateway.
func NewRazorpayGateway(keyID, keySecret, webhookSecret string) *RazorpayGateway {
	return &RazorpayGateway{
		keyID:         keyID,
		keySecret:     keySecret,
		webhookSecret: webhookSecret,
		httpClient:    &http.Client{},
	}
}

func (g *RazorpayGateway) GatewayName() string {
	return "razorpay"
}

// CreateOrder calls POST /v1/orders and returns a Razorpay order.
// Amount is converted from INR (float64) to paise (int64) as Razorpay requires.
func (g *RazorpayGateway) CreateOrder(ctx context.Context, amount float64, currency string, metadata map[string]string) (*Order, error) {
	// Razorpay requires amount in smallest currency unit (paise for INR)
	amountInPaise := int64(math.Round(amount * 100))

	receiptID := metadata["transaction_id"]
	if len(receiptID) > 40 {
		receiptID = receiptID[:40] // Razorpay receipt max length is 40
	}

	payload := map[string]interface{}{
		"amount":   amountInPaise,
		"currency": currency,
		"receipt":  receiptID,
		"notes":    metadata,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("razorpay: failed to marshal order payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, razorpayOrdersURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("razorpay: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(g.keyID, g.keySecret)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("razorpay: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("razorpay: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("razorpay: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID       string `json:"id"`
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("razorpay: failed to parse response: %w", err)
	}

	return &Order{
		ID:       result.ID,
		Amount:   float64(result.Amount) / 100, // convert back to INR
		Currency: result.Currency,
		Status:   result.Status,
		KeyID:    g.keyID, // returned so frontend can open Razorpay checkout
		MetaData: metadata,
	}, nil
}

// VerifyWebhook validates the X-Razorpay-Signature header using HMAC-SHA256.
// Razorpay signs the raw request body with the webhook secret.
// See: https://razorpay.com/docs/webhooks/validate-test/
//
// If RAZORPAY_WEBHOOK_SECRET is not set, signature verification is SKIPPED.
// This is acceptable for local development with test keys.
// In production, always set RAZORPAY_WEBHOOK_SECRET.
func (g *RazorpayGateway) VerifyWebhook(payload []byte, signature string) (bool, error) {
	if g.webhookSecret == "" {
		// Dev mode: no secret configured, trust the payload
		return true, nil
	}

	mac := hmac.New(sha256.New, []byte(g.webhookSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return false, nil
	}
	return true, nil
}
