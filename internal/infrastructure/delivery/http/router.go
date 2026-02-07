// Package httprouter provides a custom HTTP router with middleware support.
package httprouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"daunrodo/internal/config"
	"daunrodo/internal/consts"
	"daunrodo/internal/errs"
	"daunrodo/internal/infrastructure/delivery/http/middleware"
	"daunrodo/internal/infrastructure/delivery/http/request"
	"daunrodo/internal/infrastructure/delivery/http/response"
	"daunrodo/internal/service"
	"daunrodo/internal/storage"

	"github.com/google/uuid"
)

// Chain represents a sequence of middleware functions.
type Chain []func(http.Handler) http.Handler

// ThenFunc applies the middleware chain to the final handler.
func (c Chain) ThenFunc(h http.HandlerFunc) http.Handler {
	return c.then(h)
}

func (c Chain) then(h http.Handler) http.Handler {
	for _, mw := range slices.Backward(c) {
		h = mw(h)
	}

	return h
}

// Router is a custom HTTP router with middleware support.
type Router struct {
	*http.ServeMux

	log         *slog.Logger
	cfg         *config.Config
	globalChain []func(http.Handler) http.Handler
	routeChain  []func(http.Handler) http.Handler
	isSubRouter bool
	svc         service.JobManager
	storer      storage.Storer
}

// New creates a new Router instance.
func New(log *slog.Logger, cfg *config.Config, svc service.JobManager, storer storage.Storer) *Router {
	router := &Router{
		ServeMux: http.NewServeMux(),
		log:      log,
		cfg:      cfg,
		svc:      svc,
		storer:   storer,
	}

	router.SetGlobalMiddlewares()
	router.SetRoutes()

	return router
}

// Use adds middleware to the router's middleware chain.
func (ro *Router) Use(middleware ...func(http.Handler) http.Handler) {
	if ro.isSubRouter {
		ro.routeChain = append(ro.routeChain, middleware...)
	} else {
		ro.globalChain = append(ro.globalChain, middleware...)
	}
}

// Group creates a sub-router with its own middleware chain.
func (ro *Router) Group(fn func(r *Router)) {
	subRouter := &Router{
		isSubRouter: true,
		routeChain:  slices.Clone(ro.routeChain),
		ServeMux:    ro.ServeMux}

	fn(subRouter)
}

// HandleFunc registers a new route with a handler function.
func (ro *Router) HandleFunc(pattern string, h http.HandlerFunc) {
	ro.Handle(pattern, h)
}

// Handle registers a new route with a handler.
func (ro *Router) Handle(pattern string, h http.Handler) {
	for _, middleware := range slices.Backward(ro.routeChain) {
		h = middleware(h)
	}

	ro.ServeMux.Handle(pattern, h)
}

// ServeHTTP implements the http.Handler interface for the Router.
func (ro *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var handler http.Handler = ro.ServeMux

	for _, middleware := range slices.Backward(ro.globalChain) {
		handler = middleware(handler)
	}

	handler.ServeHTTP(w, req)
}

// SetGlobalMiddlewares sets the global middlewares for the router.
func (ro *Router) SetGlobalMiddlewares() {
	ro.Use(
		middleware.Recoverer,
		middleware.RequestID,
		middleware.Logger,
	)
}

// SetRoutes sets up all the routes for the router.
func (ro *Router) SetRoutes() {
	ro.SetRoutesHealthcheck()
	ro.SetRoutesJob()
	ro.SetRoutesFiles()
}

// SetRoutesHealthcheck sets up healthcheck routes.
func (ro *Router) SetRoutesHealthcheck() {
	healthcheckRouter := &Router{
		ServeMux: http.NewServeMux(),
	}
	healthcheckRouter.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	ro.Handle("/v1/", http.StripPrefix("/v1", healthcheckRouter))
}

// SetRoutesJob sets up job-related routes.
func (ro *Router) SetRoutesJob() {
	jobRouter := &Router{
		ServeMux: http.NewServeMux(),
	}
	jobRouter.HandleFunc("POST /enqueue", ro.Enqueue)
	jobRouter.HandleFunc("GET /", ro.GetJobs)
	jobRouter.HandleFunc("GET /{id}", ro.GetJob)
	jobRouter.HandleFunc("DELETE /{id}", ro.CancelJob)

	ro.Handle("/v1/jobs/", http.StripPrefix("/v1/jobs", jobRouter))
}

// SetRoutesFiles sets up file-related routes.
func (ro *Router) SetRoutesFiles() {
	fileRouter := &Router{
		ServeMux: http.NewServeMux(),
	}
	fileRouter.HandleFunc("GET /{id}", ro.DownloadPublication)

	ro.Handle("/v1/files/", http.StripPrefix("/v1/files", fileRouter))
}

// Enqueue handles job enqueue requests.
func (ro *Router) Enqueue(w http.ResponseWriter, r *http.Request) {
	log := ro.log.With("handler", "Enqueue")
	ctx := r.Context()

	var req request.Enqueue
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.ErrorContext(ctx, consts.RespInvalidRequestBody, slog.Any("error", err))
		response.BadRequest(w, consts.RespInvalidRequestBody, err)

		return
	}

	if err := req.Validate(); err != nil {
		log.ErrorContext(ctx, consts.RespUnprocessableEntity, slog.Any("error", err))
		response.UnprocessableEntity(w, consts.RespUnprocessableEntity, err)

		return
	}

	job, err := ro.svc.Enqueue(ctx, req.URL, req.Preset)
	if errors.Is(err, errs.ErrJobAlreadyExists) {
		log.DebugContext(ctx, consts.RespJobAlreadyExists, slog.Any("error", err))
		response.OK(w, consts.RespJobAlreadyExists, job.UUID, nil)

		return
	}

	if err != nil {
		log.ErrorContext(ctx, consts.RespJobEnqueueFail, slog.Any("error", err))
		response.InternalServerError(w, consts.RespJobEnqueueFail, nil, err)

		return
	}

	log.InfoContext(ctx, consts.RespJobEnqueued, slog.String("url", job.URL))

	response.Accepted(w, consts.RespJobEnqueued, job.UUID, nil)
}

// GetJob handles requests to retrieve a specific job by ID.
func (ro *Router) GetJob(w http.ResponseWriter, r *http.Request) {
	log := ro.log.With("handler", "GetJob")

	ctx, cancel := context.WithTimeout(r.Context(), consts.DefaultHandlerTimeout)
	defer cancel()

	jobID := r.PathValue("id")
	if jobID == "" || uuid.Validate(jobID) != nil {
		log.ErrorContext(ctx, consts.RespQueryParamMissing)
		response.BadRequest(w, consts.RespQueryParamMissing, nil)

		return
	}

	job, ok := ro.storer.GetJobByID(ctx, jobID)
	if !ok {
		log.ErrorContext(ctx, consts.RespJobNotFound)
		response.NoContent(w)

		return
	}

	response.OK(w, consts.RespJobRetrieved, job, nil)
}

// GetJobs handles requests to retrieve all jobs.
func (ro *Router) GetJobs(w http.ResponseWriter, r *http.Request) {
	log := ro.log.With("handler", "GetJobs")

	ctx, cancel := context.WithTimeout(r.Context(), consts.DefaultHandlerTimeout)
	defer cancel()

	jobs, err := ro.storer.GetJobs(ctx)
	if errors.Is(err, errs.ErrNoJobs) {
		log.DebugContext(ctx, consts.RespNoJobs)
		response.NoContent(w)

		return
	}

	if err != nil {
		log.ErrorContext(ctx, consts.RespGetJobsFail, slog.Any("error", err))
		response.InternalServerError(w, consts.RespGetJobsFail, nil, err)

		return
	}

	response.OK(w, consts.RespJobsRetrieved, jobs, nil)
}

// CancelJob handles job cancellation requests.
func (ro *Router) CancelJob(w http.ResponseWriter, r *http.Request) {
	log := ro.log.With("handler", "CancelJob")

	ctx, cancel := context.WithTimeout(r.Context(), consts.DefaultHandlerTimeout)
	defer cancel()

	jobID := r.PathValue("id")
	if jobID == "" || uuid.Validate(jobID) != nil {
		log.ErrorContext(ctx, consts.RespQueryParamMissing)
		response.BadRequest(w, consts.RespQueryParamMissing, nil)

		return
	}

	err := ro.storer.CancelJob(ctx, jobID)
	if errors.Is(err, errs.ErrJobNotFound) {
		log.ErrorContext(ctx, consts.RespJobNotFound, slog.Any("error", err))
		response.NotFound(w, consts.RespJobNotFound, err)

		return
	}

	if errors.Is(err, errs.ErrJobCancelled) {
		log.DebugContext(ctx, consts.RespJobCancelFailed, slog.Any("error", err))
		response.OK(w, consts.RespJobCancelled, nil, nil)

		return
	}

	if err != nil {
		log.ErrorContext(ctx, consts.RespJobCancelFailed, slog.Any("error", err))
		response.InternalServerError(w, consts.RespJobCancelFailed, nil, err)

		return
	}

	log.InfoContext(ctx, consts.RespJobCancelled, slog.String("job_id", jobID))

	response.OK(w, consts.RespJobCancelled, nil, nil)
}

// DownloadPublication handles file downloads with resume support.
func (ro *Router) DownloadPublication(w http.ResponseWriter, r *http.Request) {
	log := ro.log.With("handler", "DownloadPublication")

	ctx, cancel := context.WithTimeout(r.Context(), ro.cfg.HTTP.DownloadTimeout) // Longer timeout for downloads
	defer cancel()

	pubID := r.PathValue("id")
	if pubID == "" {
		log.ErrorContext(ctx, consts.RespQueryParamMissing)
		response.BadRequest(w, consts.RespQueryParamMissing, nil)

		return
	}

	publication, err := ro.storer.GetPublicationByID(ctx, pubID)
	if errors.Is(err, errs.ErrPublicationNotFound) {
		log.ErrorContext(ctx, consts.RespPublicationNotFound, slog.Any("error", err))
		response.NotFound(w, consts.RespPublicationNotFound, err)

		return
	}

	if err != nil || publication == nil {
		log.ErrorContext(ctx, consts.RespPublicationDownloadFailed, slog.Any("error", err))
		response.InternalServerError(w, consts.RespPublicationDownloadFailed, nil, err)

		return
	}

	file, err := os.Open(publication.Filename)
	if err != nil {
		log.ErrorContext(ctx, consts.RespFileNotFound, slog.Any("error", err))
		response.NotFound(w, consts.RespFileNotFound, err)

		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		log.ErrorContext(ctx, "file stat", slog.Any("error", err))
		response.InternalServerError(w, "file stat", nil, err)

		return
	}

	fileName := filepath.Base(publication.Filename)

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))

	http.ServeContent(w, r, fileName, fileInfo.ModTime(), file)
}
