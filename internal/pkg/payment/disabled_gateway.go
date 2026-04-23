package payment

import (
	"context"
	"fmt"
)

// DisabledGateway is used when PAYMENTS_ENABLED=false.
// All operations return a clear error. This lets the feature be toggled
// from a single environment variable without changing any code.
type DisabledGateway struct{}

func NewDisabledGateway() *DisabledGateway {
	return &DisabledGateway{}
}

func (g *DisabledGateway) GatewayName() string {
	return "razorpay" // keeps DB enum valid; no rows are ever written in disabled mode
}

func (g *DisabledGateway) CreateOrder(_ context.Context, _ float64, _ string, _ map[string]string) (*Order, error) {
	return nil, fmt.Errorf("payments are currently disabled")
}

func (g *DisabledGateway) VerifyWebhook(_ []byte, _ string) (bool, error) {
	return false, fmt.Errorf("payments are currently disabled")
}
