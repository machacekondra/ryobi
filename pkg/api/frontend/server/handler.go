package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
)

// ControllerFactoryFunc creates a Controller from Options.
type ControllerFactoryFunc func(ctrl.Options) (ctrl.Controller, error)

// HandlerOptions represents a controller to be registered with the server.
type HandlerOptions struct {
	ParentRouter      chi.Router
	Path              string
	ResourceType      string
	Method            v1.OperationMethod
	ControllerFactory ControllerFactoryFunc
	Middlewares       []func(http.Handler) http.Handler
}

// NewSubrouter creates a new subrouter and mounts it on the parent router.
func NewSubrouter(parent chi.Router, path string, middlewares ...func(http.Handler) http.Handler) chi.Router {
	subrouter := chi.NewRouter()
	parent.Mount(path, subrouter)
	subrouter.Use(middlewares...)
	return subrouter
}

// HandlerForController wraps a Controller in an http.HandlerFunc.
func HandlerForController(controller ctrl.Controller, operationType v1.OperationType) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		response, err := controller.Run(ctx, w, req)
		if err != nil {
			HandleError(ctx, w, req, err)
			return
		}

		if response != nil {
			err = response.Apply(ctx, w, req)
			if err != nil {
				HandleError(ctx, w, req, err)
				return
			}
		}
	}
}

// RegisterHandler registers a handler for the given resource type and method.
func RegisterHandler(ctx context.Context, opts HandlerOptions, ctrlOpts ctrl.Options) error {
	logger := logr.FromContextOrDiscard(ctx)

	if opts.ResourceType == "" || opts.Method == "" {
		return fmt.Errorf("resource type and method must be specified")
	}

	ctrlOpts.ResourceType = opts.ResourceType

	controller, err := opts.ControllerFactory(ctrlOpts)
	if err != nil {
		return err
	}

	operationType := v1.OperationType{Type: opts.ResourceType, Method: opts.Method}

	path := opts.Path
	if path == "" {
		path = "/"
	}

	handler := HandlerForController(controller, operationType)
	namedRouter := opts.ParentRouter.With(opts.Middlewares...)

	httpMethod := opts.Method.HTTPMethod()
	namedRouter.MethodFunc(httpMethod, path, handler)

	logger.Info("Registered handler", "method", httpMethod, "path", path, "resource", opts.ResourceType)
	return nil
}

// HandleError handles unhandled errors from controllers.
func HandleError(ctx context.Context, w http.ResponseWriter, req *http.Request, err error) {
	logger := logr.FromContextOrDiscard(ctx)
	logger.Error(err, "unhandled error")

	response := v1.NewInternalErrorResponse(err)
	applyErr := response.Apply(ctx, w, req)
	if applyErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error(applyErr, "error writing error response")
	}
}
