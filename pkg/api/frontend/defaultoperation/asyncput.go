package defaultoperation

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/components/database"
)

// DefaultAsyncPut handles PUT operations that queue async work.
// It saves the resource to the database and queues an async operation.
type DefaultAsyncPut[T any] struct {
	ctrl.BaseController
	opts      ctrl.ResourceOptions[T]
	rootScope string
}

// NewDefaultAsyncPutFactory creates a factory for DefaultAsyncPut controllers.
func NewDefaultAsyncPutFactory[T any](rootScope string, opts ctrl.ResourceOptions[T]) func(ctrl.Options) (ctrl.Controller, error) {
	return func(ctrlOpts ctrl.Options) (ctrl.Controller, error) {
		return &DefaultAsyncPut[T]{
			BaseController: ctrl.NewBaseController(ctrlOpts),
			opts:           opts,
			rootScope:      rootScope,
		}, nil
	}
}

func (c *DefaultAsyncPut[T]) Run(ctx context.Context, w http.ResponseWriter, req *http.Request) (v1.Response, error) {
	name := chi.URLParam(req, "name")
	if name == "" {
		return v1.NewBadRequestResponse("name is required"), nil
	}

	body, err := io.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		return v1.NewBadRequestResponse("failed to read request body"), nil
	}

	// Convert request body to typed resource
	newResource, err := c.opts.RequestConverter(body)
	if err != nil {
		return v1.NewBadRequestResponse("invalid request body: " + err.Error()), nil
	}

	// Build resource ID
	id := buildResourceID(c.rootScope, c.ResourceType(), name, req)

	// Check for existing resource
	var oldResource T
	etag, getErr := c.GetResource(ctx, id, &oldResource)

	isNew := errors.Is(getErr, &database.ErrNotFound{})
	if getErr != nil && !isNew {
		return nil, getErr
	}

	// Run update filters
	for _, filter := range c.opts.UpdateFilters {
		var oldPtr *T
		if !isNew {
			oldPtr = &oldResource
		}
		resp, err := filter(ctx, newResource, oldPtr, &ctrl.Options{
			Address:        c.Options().Address,
			DatabaseClient: c.DatabaseClient(),
			StatusManager:  c.StatusManager(),
			ResourceType:   c.ResourceType(),
		})
		if err != nil {
			return nil, err
		}
		if resp != nil {
			return resp, nil
		}
	}

	// Save resource to database
	saveOpts := ""
	if !isNew {
		saveOpts = etag
	}
	_, err = c.SaveResource(ctx, id, newResource, saveOpts)
	if err != nil {
		return nil, err
	}

	// Queue async operation
	operationID := uuid.New().String()
	timeout := c.opts.AsyncOperationTimeout
	if timeout == 0 {
		timeout = v1.DefaultRetryAfter * 120 // 10 minutes
	}

	operationType := v1.OperationType{Type: c.ResourceType(), Method: v1.OperationPut}
	operationURL, err := c.StatusManager().QueueAsyncOperation(ctx, operationID, id, operationType, timeout)
	if err != nil {
		return nil, err
	}

	return v1.NewAcceptedResponse(operationURL), nil
}

// DefaultAsyncDelete handles DELETE operations that queue async work.
type DefaultAsyncDelete[T any] struct {
	ctrl.BaseController
	opts      ctrl.ResourceOptions[T]
	rootScope string
}

// NewDefaultAsyncDeleteFactory creates a factory for DefaultAsyncDelete controllers.
func NewDefaultAsyncDeleteFactory[T any](rootScope string, opts ctrl.ResourceOptions[T]) func(ctrl.Options) (ctrl.Controller, error) {
	return func(ctrlOpts ctrl.Options) (ctrl.Controller, error) {
		return &DefaultAsyncDelete[T]{
			BaseController: ctrl.NewBaseController(ctrlOpts),
			opts:           opts,
			rootScope:      rootScope,
		}, nil
	}
}

func (c *DefaultAsyncDelete[T]) Run(ctx context.Context, w http.ResponseWriter, req *http.Request) (v1.Response, error) {
	name := chi.URLParam(req, "name")
	if name == "" {
		return v1.NewBadRequestResponse("name is required"), nil
	}

	id := buildResourceID(c.rootScope, c.ResourceType(), name, req)

	// Get existing resource
	var resource T
	_, err := c.GetResource(ctx, id, &resource)
	if err != nil {
		if errors.Is(err, &database.ErrNotFound{}) {
			return v1.NewNoContentResponse(), nil
		}
		return nil, err
	}

	// Run delete filters
	for _, filter := range c.opts.DeleteFilters {
		opts := &ctrl.Options{
			Address:        c.Options().Address,
			DatabaseClient: c.DatabaseClient(),
			StatusManager:  c.StatusManager(),
			ResourceType:   c.ResourceType(),
		}
		resp, err := filter(ctx, &resource, opts)
		if err != nil {
			return nil, err
		}
		if resp != nil {
			return resp, nil
		}
	}

	// Queue async operation
	operationID := uuid.New().String()
	operationType := v1.OperationType{Type: c.ResourceType(), Method: v1.OperationDelete}
	operationURL, err := c.StatusManager().QueueAsyncOperation(ctx, operationID, id, operationType, 0)
	if err != nil {
		return nil, err
	}

	return v1.NewAcceptedResponse(operationURL), nil
}

// buildResourceID constructs a resource ID from URL parameters.
func buildResourceID(rootScope, resourceType, name string, req *http.Request) string {
	appName := chi.URLParam(req, "appName")
	if appName != "" {
		return rootScope + "/ryobi/applications/" + appName + "/" + resourceType + "/" + name
	}
	return rootScope + "/" + resourceType + "/" + name
}
