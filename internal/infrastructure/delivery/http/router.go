package httprouter

import (
	"context"
	"daunrodo/internal/consts"
	"daunrodo/internal/errs"
	"daunrodo/internal/infrastructure/delivery/http/middleware"
	"daunrodo/internal/infrastructure/delivery/http/request"
	"daunrodo/internal/infrastructure/delivery/http/response"
	"daunrodo/internal/service"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"
)

type chain []func(http.Handler) http.Handler

func (c chain) thenFunc(h http.HandlerFunc) http.Handler {
	return c.then(h)
}

func (c chain) then(h http.Handler) http.Handler {
	for _, mw := range slices.Backward(c) {
		h = mw(h)
	}
	return h
}

type Router struct {
	*http.ServeMux
	log         *slog.Logger
	globalChain []func(http.Handler) http.Handler
	routeChain  []func(http.Handler) http.Handler
	isSubRouter bool
	svc         service.Job
}

func New(log *slog.Logger, svc service.Job) *Router {
	r := &Router{
		ServeMux: http.NewServeMux(),
		log:      log,
		svc:      svc,
	}

	r.SetGlobalMiddlewares()
	r.SetRoutes()

	return r
}

func (r *Router) Use(middleware ...func(http.Handler) http.Handler) {
	if r.isSubRouter {
		r.routeChain = append(r.routeChain, middleware...)
	} else {
		r.globalChain = append(r.globalChain, middleware...)
	}
}

func (r *Router) Group(fn func(r *Router)) {
	subRouter := &Router{
		isSubRouter: true,
		routeChain:  slices.Clone(r.routeChain),
		ServeMux:    r.ServeMux}

	fn(subRouter)
}

func (r *Router) HandleFunc(pattern string, h http.HandlerFunc) {
	r.Handle(pattern, h)
}

func (r *Router) Handle(pattern string, h http.Handler) {
	for _, middleware := range slices.Backward(r.routeChain) {
		h = middleware(h)
	}
	r.ServeMux.Handle(pattern, h)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var h http.Handler = r.ServeMux

	for _, middleware := range slices.Backward(r.globalChain) {
		h = middleware(h)
	}

	h.ServeHTTP(w, req)
}

func (r *Router) SetGlobalMiddlewares() {
	r.Use(
		middleware.Recoverer,
		middleware.RequestID,
		middleware.Logger,
	)
}

func (r *Router) SetRoutes() {
	r.SetRoutesHealthcheck()
	r.SetRoutesJob()
	r.SetRoutesFiles()
}

func (r *Router) SetRoutesHealthcheck() {
	healthcheckRouter := &Router{
		ServeMux: http.NewServeMux(),
	}
	healthcheckRouter.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Handle("/v1/", http.StripPrefix("/v1", healthcheckRouter))
}

func (ro *Router) SetRoutesJob() {
	jobRouter := &Router{
		ServeMux: http.NewServeMux(),
	}
	jobRouter.HandleFunc("POST /enqueue", ro.Enqueue)
	jobRouter.HandleFunc("GET /", ro.GetJobs)
	jobRouter.HandleFunc("GET /{id}", ro.GetJob)
	jobRouter.HandleFunc("DELETE /{id}/cancel", ro.CancelJob)

	ro.Handle("/v1/jobs/", http.StripPrefix("/v1/jobs", jobRouter))
}

func (ro *Router) SetRoutesFiles() {
	fileRouter := &Router{
		ServeMux: http.NewServeMux(),
	}
	fileRouter.HandleFunc("POST /enqueue", ro.Enqueue)
	fileRouter.HandleFunc("GET /", ro.GetJobs)
	fileRouter.HandleFunc("GET /{id}", ro.GetJob)
	fileRouter.HandleFunc("DELETE /{id}/cancel", ro.CancelJob)

	ro.Handle("/v1/files/", http.StripPrefix("/v1/files", fileRouter))
}

func (ro *Router) Enqueue(w http.ResponseWriter, r *http.Request) {
	log := ro.log.With("handler", "Enqueue")
	ctx := r.Context()

	var in request.Enqueue
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		log.ErrorContext(ctx, consts.RespInvalidRequestBody, slog.Any("error", err))
		response.BadRequest(w, consts.RespInvalidRequestBody, err)

		return
	}

	if err := in.Validate(); err != nil {
		log.ErrorContext(ctx, consts.RespUnprocessableEntity, slog.Any("error", err))
		response.UnprocessableEntity(w, consts.RespUnprocessableEntity, err)

		return
	}

	job, err := ro.svc.Enqueue(ctx, in.URL, in.Preset)
	if errors.Is(err, errs.ErrJobAlreadyExists) {
		log.DebugContext(ctx, consts.RespJobAlreadyExists, slog.Any("error", err))
		response.OK(w, consts.RespJobAlreadyExists, nil, nil)

		return
	}
	if err != nil {
		log.ErrorContext(ctx, consts.RespJobEnqueueFail, slog.Any("error", err))
		response.InternalServerError(w, consts.RespJobEnqueueFail, nil, err)

		return
	}

	log.InfoContext(ctx, consts.RespJobEnqueued, slog.String("url", job.URL))

	response.Accepted(w, consts.RespJobEnqueued, job.ID, nil)
}

func (ro *Router) GetJob(w http.ResponseWriter, r *http.Request) {
	log := ro.log.With("handler", "GetJob")

	ctx, cancel := context.WithTimeout(r.Context(), consts.DefaultHandlerTimeout)
	defer cancel()

	id := r.PathValue("id")
	if id == "" {
		log.ErrorContext(ctx, consts.RespQueryParamMissing)
		response.BadRequest(w, consts.RespQueryParamMissing, nil)

		return
	}

	job := ro.svc.GetByID(ctx, id)
	if job == nil {
		log.ErrorContext(ctx, consts.RespJobNotFound)
		response.NoContent(w)

		return
	}

	response.OK(w, consts.RespJobRetrieved, job, nil)
}

func (ro *Router) GetJobs(w http.ResponseWriter, r *http.Request) {
	log := ro.log.With("handler", "GetJobs")

	ctx, cancel := context.WithTimeout(r.Context(), consts.DefaultHandlerTimeout)
	defer cancel()

	jobs, err := ro.svc.GetAll(ctx)
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

func (ro *Router) CancelJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	r.PathValue("job_id")

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	// err := ro.svc.Stop(ctx)
	// if err != nil {
	// 	ro.log.ErrorContext(ctx, "service stop failed", slog.Any("error", err))
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	w.Write([]byte("service stop failed"))
	// 	return
	// }
	response.OK(w, "service stopped", nil, nil)
}
