// Package middleware provides HTTP middleware for the chi router.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"namenotdecidedyet/internal/pkg/apierror"
	"namenotdecidedyet/internal/pkg/respond"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey is an unexported type for context keys to prevent collisions
// with keys from other packages.
type contextKey string

const (
	// ContextKeyUserID is the context key for the authenticated user's UUID string.
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyRole is the context key for the authenticated user's role string.
	ContextKeyRole contextKey = "role"
)

// Auth returns a middleware that validates the Bearer JWT in the Authorization header.
// It only accepts tokens with token_type = "access" — refresh tokens are rejected.
// On success: injects user_id and role into the request context and calls next.
// On failure: responds with 401 and halts the chain.
func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respond.Error(w, apierror.Unauthorized("missing authorization header"))
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				respond.Error(w, apierror.Unauthorized("authorization header format must be: Bearer <token>"))
				return
			}

			token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, apierror.Unauthorized("unexpected token signing method")
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				respond.Error(w, apierror.Unauthorized("invalid or expired token"))
				return
			}

			mc, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				respond.Error(w, apierror.Unauthorized("malformed token claims"))
				return
			}

			// Reject refresh tokens on protected routes — they must only be used at /auth/refresh
			tokenType, _ := mc["token_type"].(string)
			if tokenType != "access" {
				respond.Error(w, apierror.Unauthorized("access token required"))
				return
			}

			userID, _ := mc["user_id"].(string)
			role, _ := mc["role"].(string)

			ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
			ctx = context.WithValue(ctx, ContextKeyRole, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns a middleware that gates access to one or more allowed roles.
// MUST be used after the Auth middleware (depends on role being in context).
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, r := range allowedRoles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := RoleFromContext(r.Context())
			if role == "" {
				respond.Error(w, apierror.Unauthorized("role not found in token"))
				return
			}
			if _, ok := allowed[role]; !ok {
				respond.Error(w, apierror.Forbidden("you do not have permission to access this resource"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserIDFromContext extracts the authenticated user's UUID string from the context.
// Returns an empty string if Auth middleware has not run.
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ContextKeyUserID).(string)
	return id
}

// RoleFromContext extracts the authenticated user's role from the context.
// Returns an empty string if Auth middleware has not run.
func RoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(ContextKeyRole).(string)
	return role
}
