package httpserver

import (
	"context"
	"net/http"
	"time"
)

const (
	defaultReadTimeout     = 5 * time.Second
	defaultWriteTimeout    = 5 * time.Second
	defaultAddr            = ":80"
	defaultShutdownTimeout = 3 * time.Second
)

type Server struct {
	server          *http.Server
	errCh           chan error
	shutdownTimeout time.Duration
}

type Options struct {
	Addr            string
	ShutdownTimeout time.Duration
}

func New(handler http.Handler, opt Options) *Server {
	httpServer := &http.Server{
		Handler: handler,
		Addr:    defaultAddr,
	}

	srv := &Server{
		server:          httpServer,
		errCh:           make(chan error, 10),
		shutdownTimeout: opt.ShutdownTimeout,
	}

	go srv.start()

	return srv
}

func (s *Server) start() {
	s.errCh <- s.server.ListenAndServe()
	close(s.errCh)
}

// func (s *Server) Notify() <-chan error {
// 	return s.errCh
// }

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	return s.server.Shutdown(ctx)
}
