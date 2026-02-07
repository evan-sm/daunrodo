// Package middleware provides HTTP middleware for request handling, logging, and recovery.
package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"daunrodo/internal/observability"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type contextKey string

// RequestIDKey is the context key for storing the request ID.
const RequestIDKey contextKey = "requestID"

const (
	// HeaderXRequestID is the HTTP header for the request ID.
	HeaderXRequestID = "X-Request-ID" // https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Request-ID
)

// RequestLog represents the log structure for an HTTP request.
type RequestLog struct {
	Method        string `json:"method"`
	URI           string `json:"uri"`
	RemoteAddr    string `json:"remoteAddr"`
	Proto         string `json:"proto"`
	ContentLength int64  `json:"contentLength"`
}

// Recoverer is a middleware that recovers from panics in HTTP handlers.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(ctx context.Context) {
			if rvr := recover(); rvr != nil {
				slog.ErrorContext(ctx, "middleware: panic recovered", slog.Any("error", rvr))

				if rvr == http.ErrAbortHandler {
					panic(rvr)
				}
			}
		}(r.Context())

		next.ServeHTTP(w, r)
	})
}

// RequestID sets a unique request ID for each incoming HTTP request.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(HeaderXRequestID)
		if reqID == "" {
			reqID = uuid.NewString()
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
		w.Header().Set(HeaderXRequestID, reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logger logs incoming HTTP requests.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.DebugContext(r.Context(), "http request",
			slog.Any("request", RequestLog{
				Method:        r.Method,
				URI:           r.RequestURI,
				RemoteAddr:    r.RemoteAddr,
				Proto:         r.Proto,
				ContentLength: r.ContentLength,
			}))
		next.ServeHTTP(w, r)
	})
}

// Prometheus instruments HTTP requests using Prometheus native middleware.
func Prometheus(metrics *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if metrics == nil {
			return next
		}

		handler := promhttp.InstrumentHandlerResponseSize(metrics.HTTPResponseSize, next)
		handler = promhttp.InstrumentHandlerDuration(metrics.HTTPRequestDuration, handler)
		handler = promhttp.InstrumentHandlerCounter(metrics.HTTPRequestsTotal, handler)
		handler = promhttp.InstrumentHandlerInFlight(metrics.HTTPInFlight, handler)

		return handler
	}
}
