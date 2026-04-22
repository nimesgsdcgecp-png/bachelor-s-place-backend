package embedding

import "context"

// Provider defines the standard interface for all AI embedding services.
type Provider interface {
	Generate(ctx context.Context, text string) ([]float32, error)
}
