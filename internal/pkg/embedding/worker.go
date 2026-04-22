package embedding

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type Worker struct {
	pool     *pgxpool.Pool
	provider Provider
}

func NewWorker(pool *pgxpool.Pool, provider Provider) *Worker {
	return &Worker{
		pool:     pool,
		provider: provider,
	}
}

func (w *Worker) Start(ctx context.Context) {
	go func() {
		log.Info().Msg("embedding worker started")
			
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.processPending(ctx)
			}
		}
	}()
}

func (w *Worker) processPending(ctx context.Context) {
	const findQuery = `
		SELECT id, lifestyle_tags, bio
		FROM users
		WHERE pending_embeddings = TRUE
		  AND deleted_at IS NULL
		LIMIT 5`

	rows, err := w.pool.Query(ctx, findQuery)
	if err != nil {
		log.Error().Err(err).Msg("failed to query pending users")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var tags []string
		var bio *string
		if err := rows.Scan(&id, &tags, &bio); err != nil {
			continue
		}

		// Prepare text for embedding: combine tags and bio
		text := strings.Join(tags, ", ")
		if bio != nil {
			text = text + ". " + *bio
		}

		if text == "" {
			text = "lifestyle profile"
		}

		embedding, err := w.provider.Generate(ctx, text)
		if err != nil {
			log.Error().Err(err).Str("user_id", id).Msg("failed to get embedding")
			continue
		}

		// Format embedding as pgvector string: [0.1,0.2,...]
		strParts := make([]string, len(embedding))
		for i, v := range embedding {
			strParts[i] = fmt.Sprintf("%g", v)
		}
		vectorStr := "[" + strings.Join(strParts, ",") + "]"

		// Update user with the new vector
		const updateQuery = `
			UPDATE users
			SET personality_embedding = $1, pending_embeddings = FALSE
			WHERE id = $2`

		if _, err := w.pool.Exec(ctx, updateQuery, vectorStr, id); err != nil {
			log.Error().Err(err).Str("user_id", id).Msg("failed to save embedding")
		} else {
			log.Info().Str("user_id", id).Msg("successfully updated personality embedding")
		}
	}
}
