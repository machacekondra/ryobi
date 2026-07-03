package gateway

import (
	"context"
	"io/fs"
	"net/http"
	"strings"

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

// MountSPA mounts a single-page application at the given path.
// The embedFS should contain a "dist" directory with index.html and assets.
func (r *Router) MountSPA(path string, embedFS fs.FS) {
	subFS, err := fs.Sub(embedFS, "dist")
	if err != nil {
		r.logger.Error(err, "Failed to mount SPA", "path", path)
		return
	}

	prefix := strings.TrimSuffix(path, "/")

	r.chi.Handle(prefix+"/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Strip prefix to get file path relative to embedded FS
		filePath := strings.TrimPrefix(req.URL.Path, prefix)
		filePath = strings.TrimPrefix(filePath, "/")

		if filePath == "" {
			filePath = "index.html"
		}

		// Serve the file if it exists, otherwise serve index.html (SPA fallback)
		data, err := fs.ReadFile(subFS, filePath)
		if err != nil {
			data, err = fs.ReadFile(subFS, "index.html")
			if err != nil {
				http.NotFound(w, req)
				return
			}
			filePath = "index.html"
		}

		// Set content type
		switch {
		case strings.HasSuffix(filePath, ".html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case strings.HasSuffix(filePath, ".js"):
			w.Header().Set("Content-Type", "application/javascript")
		case strings.HasSuffix(filePath, ".css"):
			w.Header().Set("Content-Type", "text/css")
		case strings.HasSuffix(filePath, ".svg"):
			w.Header().Set("Content-Type", "image/svg+xml")
		case strings.HasSuffix(filePath, ".json"):
			w.Header().Set("Content-Type", "application/json")
		case strings.HasSuffix(filePath, ".png"):
			w.Header().Set("Content-Type", "image/png")
		case strings.HasSuffix(filePath, ".ico"):
			w.Header().Set("Content-Type", "image/x-icon")
		}

		w.Write(data)
	}))

	r.logger.Info("Mounted SPA", "path", prefix+"/")
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
