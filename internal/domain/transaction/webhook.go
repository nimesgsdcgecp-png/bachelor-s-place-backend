package transaction

import (
	"encoding/json"
	"fmt"
)

// razorpayWebhookPayload is the structure of a Razorpay webhook body.
// Razorpay sends: {"entity":"event","event":"payment.captured","payload":{"payment":{"entity":{"order_id":"order_xxx"}}}}
type razorpayWebhookPayload struct {
	Event   string `json:"event"`
	Payload struct {
		Payment struct {
			Entity struct {
				OrderID string `json:"order_id"`
			} `json:"entity"`
		} `json:"payment"`
	} `json:"payload"`
}

// extractOrderIDFromWebhook parses the raw Razorpay webhook body and returns the order_id.
// The order_id is what we store as gateway_reference_id (set during CreateOrder).
func extractOrderIDFromWebhook(body []byte) (string, error) {
	var p razorpayWebhookPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return "", fmt.Errorf("could not parse webhook body: %w", err)
	}
	orderID := p.Payload.Payment.Entity.OrderID
	if orderID == "" {
		return "", fmt.Errorf("order_id not found in webhook payload")
	}
	return orderID, nil
}
