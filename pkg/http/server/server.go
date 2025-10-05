// Package httpserver provides an HTTP server implementation.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	defaultErrChanSize = 100
)

// Server represents an HTTP server.
type Server struct {
	server          *http.Server
	errCh           chan error
	shutdownTimeout time.Duration
}

// Options contains configuration settings for the HTTP server.
type Options struct {
	Addr            string
	ShutdownTimeout time.Duration
}

// New creates a new HTTP server with the given handler and options.
func New(handler http.Handler, opt Options) *Server {
	httpServer := &http.Server{
		Handler: handler,
		Addr:    opt.Addr,
	}

	srv := &Server{
		server:          httpServer,
		errCh:           make(chan error, defaultErrChanSize),
		shutdownTimeout: opt.ShutdownTimeout,
	}

	go srv.start()

	return srv
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	err := s.server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("httpserver shutdown: %w", err)
	}

	return nil
}

func (s *Server) start() {
	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.errCh <- err
	}

	close(s.errCh)
}
