package apierror

import "net/http"

// Code is a machine-readable string identifying the error type.
// Frontend clients should switch on this value, not on HTTP status codes.
type Code string

const (
	CodeValidationError       Code = "VALIDATION_ERROR"
	CodeNotFound              Code = "NOT_FOUND"
	CodeUnauthorized          Code = "UNAUTHORIZED"
	CodeForbidden             Code = "FORBIDDEN"
	CodeBusinessRuleViolation Code = "BUSINESS_RULE_VIOLATION"
	CodeConflict              Code = "CONFLICT"
	CodeInternalError         Code = "INTERNAL_ERROR"
)

// APIError is the standard error type returned to API clients.
// Raw database errors must NEVER appear here — always map to an APIError.
type APIError struct {
	HTTPStatus int  `json:"-"` // used by respond package; never serialised
	Code       Code `json:"code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return string(e.Code) + ": " + e.Message
}

// New creates an APIError with an explicit HTTP status code.
func New(status int, code Code, message string) *APIError {
	return &APIError{HTTPStatus: status, Code: code, Message: message}
}

// Pre-built constructors for common cases.

func NotFound(msg string) *APIError {
	return New(http.StatusNotFound, CodeNotFound, msg)
}

func Unauthorized(msg string) *APIError {
	return New(http.StatusUnauthorized, CodeUnauthorized, msg)
}

func Forbidden(msg string) *APIError {
	return New(http.StatusForbidden, CodeForbidden, msg)
}

func ValidationError(msg string) *APIError {
	return New(http.StatusBadRequest, CodeValidationError, msg)
}

func BusinessRuleViolation(msg string) *APIError {
	return New(http.StatusUnprocessableEntity, CodeBusinessRuleViolation, msg)
}

func Conflict(msg string) *APIError {
	return New(http.StatusConflict, CodeConflict, msg)
}

func Internal(msg string) *APIError {
	return New(http.StatusInternalServerError, CodeInternalError, msg)
}
