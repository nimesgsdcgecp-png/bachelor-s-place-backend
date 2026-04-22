package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"namenotdecidedyet/internal/config"
	"namenotdecidedyet/internal/handler"
	"namenotdecidedyet/internal/pkg/crypto"
	"namenotdecidedyet/internal/pkg/embedding"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load .env file in development (silently ignored if the file doesn't exist)
	_ = godotenv.Load()

	// Human-readable console logging for development; JSON in production
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05"})

	// Load and validate configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	if cfg.IsProduction() {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		// Switch to JSON logging in production
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	// Connect to PostgreSQL via a connection pool
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create database connection pool")
	}
	defer pool.Close()

	// Verify the database is reachable before accepting traffic
	if err := pool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database — check DATABASE_URL and that the DB is running")
	}
	log.Info().Msg("database connection established")

	encryptor, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize encryptor")
	}

	// Build the HTTP router (pool is passed to wire up repositories)
	router := handler.NewRouter(pool, cfg.JWTSecret, encryptor)

	// Start background workers
	workerCtx, workerCancel := context.WithCancel(context.Background())
	embeddingWorker := embedding.NewWorker(pool)
	embeddingWorker.Start(workerCtx)

	// Configure the HTTP server with sensible timeouts
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the server in a goroutine so we don't block the signal listener
	go func() {
		log.Info().
			Str("addr", srv.Addr).
			Str("env", cfg.Env).
			Msg("server starting")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	// Block until we receive SIGINT or SIGTERM (Ctrl+C or Cloud Run shutdown)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutdown signal received — draining connections...")

	// Signal background workers to stop
	workerCancel()

	// Give in-flight requests up to 10 seconds to complete
	shutdownCtx, serverCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer serverCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("server forced to shutdown before drain completed")
	}

	log.Info().Msg("server exited cleanly")
}
