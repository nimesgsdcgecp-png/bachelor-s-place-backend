package handler

import (
	"net/http"

	"namenotdecidedyet/internal/domain/kyc"
	"namenotdecidedyet/internal/domain/property"
	"namenotdecidedyet/internal/domain/user"
	"namenotdecidedyet/internal/domain/squad"
	"namenotdecidedyet/internal/domain/verification"
	"namenotdecidedyet/internal/middleware"
	"namenotdecidedyet/internal/pkg/crypto"
	"namenotdecidedyet/internal/pkg/respond"
	"namenotdecidedyet/internal/repository"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewRouter builds and returns the application's HTTP handler.
func NewRouter(pool *pgxpool.Pool, jwtSecret string, encryptor *crypto.Encryptor) http.Handler {
	r := chi.NewRouter()

	// ── Setup Dependencies ───────────────────────────────────────────────────
	userRepo := repository.NewUserRepo(pool)
	userService := user.NewService(userRepo, jwtSecret)

	kycRepo := repository.NewKYCRepo(pool)
	kycService := kyc.NewService(kycRepo, userRepo, encryptor)
	
	propertyRepo := repository.NewPropertyRepo(pool)
	propertyService := property.NewService(propertyRepo, kycRepo, userRepo)
	
	verificationRepo := repository.NewVerificationRepo(pool)
	verificationService := verification.NewService(verificationRepo, propertyRepo)

	squadRepo := repository.NewSquadRepo(pool)
	squadService := squad.NewService(squadRepo)
	
	authHandler := NewAuthHandler(userService)
	userHandler := NewUserHandler(userService)
	kycHandler := NewKYCHandler(kycService)
	propertyHandler := NewPropertyHandler(propertyService)
	verificationHandler := NewVerificationHandler(verificationService)
	squadHandler := NewSquadHandler(squadService)

	// ── Global middleware (every request) ───────────────────────────────────
	r.Use(middleware.CORS)
	r.Use(middleware.Logger)
	r.Use(chimiddleware.Recoverer)  // recover from panics; log and return 500
	r.Use(chimiddleware.RequestID) // attach X-Request-Id header

	// ── Health check (no auth) ───────────────────────────────────────────────
	r.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// ── API v1 ───────────────────────────────────────────────────────────────
	r.Route("/api/v1", func(r chi.Router) {

		// ── Public routes (no JWT required) ─────────────────────────────────
		// Module 2: Auth
		r.Mount("/auth", authHandler.Routes())

		// ── Authenticated routes (JWT required) ─────────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(jwtSecret))

			// Module 3: User profile
			r.Mount("/users", userHandler.Routes())

			// Module 4: KYC (landlord submits own KYC)
			r.Mount("/kyc", kycHandler.Routes())

			// Module 5: Properties & rooms
			r.Mount("/properties", propertyHandler.Routes())

			// Module 7: Squad lookups, squads, proposals
			r.Mount("/", squadHandler.Routes())

			// Module 8: Transactions
			// r.Mount("/transactions", transactionHandler.Routes())

			// Module 9: Messages (mounted under /squads/{id}/messages in squadHandler)

			// Module 10: Notifications
			// r.Mount("/notifications", notificationHandler.Routes())
		})

		// ── Admin routes (JWT + role = admin required) ──────────────────────
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(jwtSecret))
			r.Use(middleware.RequireRole("admin"))

			// Module 4: Admin KYC approval/rejection
			r.Mount("/admin/kyc", kycHandler.AdminRoutes())

			// Module 6: Admin verification queue
			// Note: The verificationHandler.AdminRoutes() mounts both:
			// /verifications/... and /properties/...
			// so we mount it at /admin.
			r.Mount("/admin", verificationHandler.AdminRoutes())

			// Module 8: Admin transaction management
			// r.Mount("/admin/transactions", adminTransactionHandler.Routes())
		})
	})

	return r
}
