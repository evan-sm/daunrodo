package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRecoverer(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantPanic  interface{}
		wantCalled bool
		wantStatus int
	}{
		{
			name: "no panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			},
			wantPanic:  nil,
			wantCalled: true,
			wantStatus: http.StatusOK,
		},
		{
			name: "string panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("test panic")
			},
			wantPanic:  nil,
			wantCalled: true,
			wantStatus: 0,
		},
		{
			name: "error panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic(errors.New("test error panic"))
			},
			wantPanic:  nil,
			wantCalled: true,
			wantStatus: 0,
		},
		{
			name: "http.ErrAbortHandler re-panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic(http.ErrAbortHandler)
			},
			wantPanic:  http.ErrAbortHandler,
			wantCalled: true,
			wantStatus: 0,
		},
		{
			name: "panic after response started",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
				panic("test panic")
			},
			wantPanic:  nil,
			wantCalled: true,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				tt.handler(w, r)
			})

			mw := Recoverer(next)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			if tt.wantPanic != nil {
				defer func() {
					r := recover()
					if r == nil {
						t.Error("expected panic, got none")
					}
					if r != tt.wantPanic {
						t.Errorf("got panic %v, want %v", r, tt.wantPanic)
					}
				}()
			}

			mw.ServeHTTP(w, req)

			if called != tt.wantCalled {
				t.Errorf("got called %v, want %v", called, tt.wantCalled)
			}

			if tt.wantStatus != 0 {
				if got := w.Result().StatusCode; got != tt.wantStatus {
					t.Errorf("got status %v, want %v", got, tt.wantStatus)
				}
			}
		})
	}
}

func TestLogger(t *testing.T) {
	var buf bytes.Buffer

	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(log)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`done`))
	})

	mw := Logger(next)

	req := httptest.NewRequest("POST", "http://example.com/foo?bar=baz", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	req.Proto = "HTTP/1.1"
	req.ContentLength = 123
	now := time.Now()

	w := httptest.NewRecorder()

	mw.ServeHTTP(w, req)

	if !called {
		t.Error("next handler was not called")
	}

	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if body := w.Body.String(); body != "done" {
		t.Errorf("got %q, want %q", body, "done")
	}

	var logEntry struct {
		Time    time.Time  `json:"time"`
		Level   string     `json:"level"`
		Msg     string     `json:"msg"`
		Request requestLog `json:"request"`
	}

	logged := buf.String() // "{\"time\":\"2025-07-20T11:14:19.88118+03:00\",\"level\":\"DEBUG\",\"msg\":\"http_request\",\"request\":{\"method\":\"POST\",\"uri\":\"http://example.com/foo?bar=baz\",\"remote_addr\":\"1.2.3.4:1234\",\"proto\":\"HTTP/1.1\",\"content_length\":123}}\n"
	if err := json.Unmarshal([]byte(logged), &logEntry); err != nil {
		t.Errorf("failed to unmarshal log entry: %v", err)
	}

	if logEntry.Time.IsZero() || logEntry.Time.Before(now.Add(-time.Minute)) {
		t.Errorf("got %q, want non-zero", logEntry.Time)
	}

	if logEntry.Level != "DEBUG" {
		t.Errorf("got %q, want %q", logEntry.Level, "DEBUG")
	}

	if logEntry.Msg != "http request" {
		t.Errorf("got %q, want %q", logEntry.Msg, "http request")
	}

	if logEntry.Request.Method != "POST" {
		t.Errorf("got %q, want %q", logEntry.Request.Method, "POST")
	}
	if logEntry.Request.URI != "http://example.com/foo?bar=baz" {
		t.Errorf("got %q, want %q", logEntry.Request.URI, "http://example.com/foo?bar=baz")
	}
	if logEntry.Request.RemoteAddr != "1.2.3.4:1234" {
		t.Errorf("got %q, want %q", logEntry.Request.RemoteAddr, "1.2.3.4:1234")
	}
	if logEntry.Request.Proto != "HTTP/1.1" {
		t.Errorf("got %q, want %q", logEntry.Request.Proto, "HTTP/1.1")
	}
	if logEntry.Request.ContentLength != 123 {
		t.Errorf("got %q, want %q", logEntry.Request.ContentLength, 123)
	}
}

func TestRequestID(t *testing.T) {
	tests := []struct {
		name        string
		headerValue string
		validateID  func(string) bool
	}{
		{
			name:        "existing requestID",
			headerValue: "test-request-1234",
			validateID:  func(id string) bool { return id == "test-request-1234" },
		},
		{
			name:        "generated requestID",
			headerValue: "",
			validateID: func(id string) bool {
				_, err := uuid.Parse(id)
				return err == nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxChecked := false

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reqID, ok := r.Context().Value(RequestIDKey).(string)
				if !ok {
					t.Error("requestID is missing in context")
				}
				if !tt.validateID(reqID) {
					t.Errorf("requestID in context is invalid: %s", reqID)
				}

				ctxChecked = true
				w.Write([]byte("ok"))
			})

			req := httptest.NewRequest("GET", "/", nil)
			if tt.headerValue != "" {
				req.Header.Set("X-Request-ID", tt.headerValue)
			}

			w := httptest.NewRecorder()
			mw := RequestID(next)
			mw.ServeHTTP(w, req)

			if !ctxChecked {
				t.Error("next handler was not called or context was not checked")
			}

			res := w.Result()

			resID := res.Header.Get("X-Request-ID")
			if resID == "" {
				t.Error("X-Request-ID header is missing in response")
			}

			if !tt.validateID(resID) {
				t.Errorf("X-Request-ID header is invalid: %s", resID)
			}
		})
	}
}
