// Package respond provides helpers for writing consistent JSON API responses.
// All API responses use the envelope: { "success", "data", "error", "meta" }.
package respond

import (
	"encoding/json"
	"net/http"

	"namenotdecidedyet/internal/pkg/apierror"
)

type envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *errBody    `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Meta carries pagination metadata.
type Meta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

// JSON writes a 200-level success response with the given data payload.
func JSON(w http.ResponseWriter, status int, data interface{}) {
	write(w, status, envelope{Success: true, Data: data})
}

// JSONWithMeta writes a success response with pagination metadata.
func JSONWithMeta(w http.ResponseWriter, status int, data interface{}, meta *Meta) {
	write(w, status, envelope{Success: true, Data: data, Meta: meta})
}

// Error writes a structured error response.
// Accepts *apierror.APIError; falls back to 500 for any other error type.
func Error(w http.ResponseWriter, err error) {
	apiErr, ok := err.(*apierror.APIError)
	if !ok {
		apiErr = apierror.Internal("an unexpected error occurred")
	}
	write(w, apiErr.HTTPStatus, envelope{
		Success: false,
		Error:   &errBody{Code: string(apiErr.Code), Message: apiErr.Message},
	})
}

func write(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
