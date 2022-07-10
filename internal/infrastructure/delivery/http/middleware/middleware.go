package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const RequestIDKey contextKey = "requestID"

const (
	HeaderXRequestID = "X-Request-ID"
)

type requestLog struct {
	Method        string `json:"method"`
	URI           string `json:"uri"`
	RemoteAddr    string `json:"remote_addr"`
	Proto         string `json:"proto"`
	ContentLength int64  `json:"content_length"`
}

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				if rvr == http.ErrAbortHandler {
					panic(rvr)
				}

			}
		}()
		next.ServeHTTP(w, r)
	})
}

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

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.DebugContext(r.Context(), "http request",
			slog.Any("request", requestLog{
				Method:        r.Method,
				URI:           r.RequestURI,
				RemoteAddr:    r.RemoteAddr,
				Proto:         r.Proto,
				ContentLength: r.ContentLength,
			}))
		next.ServeHTTP(w, r)
	})
}
