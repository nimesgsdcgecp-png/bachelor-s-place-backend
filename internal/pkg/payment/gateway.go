package payment

import (
	"context"
)

// Order represents a payment request created on the gateway.
type Order struct {
	ID       string
	Amount   float64
	Currency string
	Status   string
	KeyID    string            // Razorpay public key — needed by the frontend to open checkout
	MetaData map[string]string
}

// Gateway defines the standard interface for payment processors (Razorpay, Stripe, etc.)
// To swap providers: implement this interface and update the router wiring in NewRouter().
type Gateway interface {
	// GatewayName returns the gateway identifier stored in the DB (must match payment_gateway enum).
	GatewayName() string

	// CreateOrder initiates a payment on the gateway and returns an Order the client uses to complete payment.
	CreateOrder(ctx context.Context, amount float64, currency string, metadata map[string]string) (*Order, error)

	// VerifyWebhook validates that a webhook notification actually came from the gateway.
	// payload is the raw request body bytes. signature is from the gateway header.
	VerifyWebhook(payload []byte, signature string) (bool, error)
}
