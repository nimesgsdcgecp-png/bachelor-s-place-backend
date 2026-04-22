package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type Worker struct {
	pool     *pgxpool.Pool
	provider string
	host     string
	model    string
	apiKey   string
}

func NewWorker(pool *pgxpool.Pool) *Worker {
	return &Worker{
		pool:     pool,
		provider: os.Getenv("EMBEDDING_PROVIDER"),
		host:     os.Getenv("OLLAMA_HOST"),
		model:    os.Getenv("EMBEDDING_MODEL"),
		apiKey:   os.Getenv("OPENAI_API_KEY"),
	}
}

func (w *Worker) Start(ctx context.Context) {
	go func() {
		log.Info().
			Str("provider", w.provider).
			Str("model", w.model).
			Msg("embedding worker started")
			
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

		embedding, err := w.getEmbedding(ctx, text)
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

func (w *Worker) getEmbedding(ctx context.Context, text string) ([]float32, error) {
	if w.provider == "ollama" {
		return w.getOllamaEmbedding(ctx, text)
	}
	// Fallback/Placeholder for OpenAI
	return nil, fmt.Errorf("unsupported or unconfigured provider: %s", w.provider)
}

func (w *Worker) getOllamaEmbedding(ctx context.Context, text string) ([]float32, error) {
	url := fmt.Sprintf("%s/api/embeddings", w.host)
	payload := map[string]string{
		"model":  w.model,
		"prompt": text,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Embedding, nil
}
