package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// Logger returns a middleware that logs each HTTP request with method, path,
// status code, and response latency using zerolog structured logging.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture the status code after the handler runs
		wrapped := &wrappedWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", wrapped.status).
			Dur("latency", time.Since(start)).
			Str("remote_addr", r.RemoteAddr).
			Str("request_id", r.Header.Get("X-Request-Id")).
			Msg("request")
	})
}

// wrappedWriter intercepts WriteHeader so we can capture the HTTP status code.
type wrappedWriter struct {
	http.ResponseWriter
	status int
}

func (ww *wrappedWriter) WriteHeader(status int) {
	ww.status = status
	ww.ResponseWriter.WriteHeader(status)
}
