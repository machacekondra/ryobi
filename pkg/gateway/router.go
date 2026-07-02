package gateway

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-logr/logr"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/api/frontend/server"
)

// Router sets up the Chi router with all API routes.
type Router struct {
	chi    chi.Router
	opts   ctrl.Options
	logger logr.Logger
}

// NewRouter creates a new Router with middleware.
func NewRouter(opts ctrl.Options, logger logr.Logger) *Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middlewareLogger(logger))
	r.Use(middleware.Recoverer)

	return &Router{
		chi:    r,
		opts:   opts,
		logger: logger,
	}
}

// Handler returns the http.Handler.
func (r *Router) Handler() http.Handler {
	return r.chi
}

// RegisterHealthCheck registers the health check endpoint.
func (r *Router) RegisterHealthCheck() {
	r.chi.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
}

// RegisterOperationStatus registers the operation status endpoint.
func (r *Router) RegisterOperationStatus(factory server.ControllerFactoryFunc) {
	ctx := logr.NewContext(context.Background(), r.logger)
	_ = server.RegisterHandler(ctx, server.HandlerOptions{
		ParentRouter:      r.chi,
		Path:              "/api/v1/operations/{operationId}",
		ResourceType:      "operations",
		Method:            v1.OperationGet,
		ControllerFactory: factory,
	}, r.opts)
}

// RegisterResourceRoutes registers CRUD routes for a resource type.
func (r *Router) RegisterResourceRoutes(basePath string, resourceType string, factories ResourceFactories) {
	ctx := logr.NewContext(context.Background(), r.logger)

	if factories.List != nil {
		_ = server.RegisterHandler(ctx, server.HandlerOptions{
			ParentRouter:      r.chi,
			Path:              basePath,
			ResourceType:      resourceType,
			Method:            v1.OperationList,
			ControllerFactory: factories.List,
		}, r.opts)
	}

	if factories.Get != nil {
		_ = server.RegisterHandler(ctx, server.HandlerOptions{
			ParentRouter:      r.chi,
			Path:              basePath + "/{name}",
			ResourceType:      resourceType,
			Method:            v1.OperationGet,
			ControllerFactory: factories.Get,
		}, r.opts)
	}

	if factories.Put != nil {
		_ = server.RegisterHandler(ctx, server.HandlerOptions{
			ParentRouter:      r.chi,
			Path:              basePath + "/{name}",
			ResourceType:      resourceType,
			Method:            v1.OperationPut,
			ControllerFactory: factories.Put,
		}, r.opts)
	}

	if factories.Delete != nil {
		_ = server.RegisterHandler(ctx, server.HandlerOptions{
			ParentRouter:      r.chi,
			Path:              basePath + "/{name}",
			ResourceType:      resourceType,
			Method:            v1.OperationDelete,
			ControllerFactory: factories.Delete,
		}, r.opts)
	}
}

// ResourceFactories holds controller factories for a resource type.
type ResourceFactories struct {
	List   server.ControllerFactoryFunc
	Get    server.ControllerFactoryFunc
	Put    server.ControllerFactoryFunc
	Delete server.ControllerFactoryFunc
}

func middlewareLogger(logger logr.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.V(1).Info("request", "method", r.Method, "path", r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}
