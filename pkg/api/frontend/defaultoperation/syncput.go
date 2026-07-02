package defaultoperation

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/components/database"
)

// DefaultSyncPut handles synchronous PUT operations with typed conversion and filters.
type DefaultSyncPut[T any] struct {
	ctrl.BaseController
	opts      ctrl.ResourceOptions[T]
	rootScope string
}

// NewDefaultSyncPutFactory creates a factory for DefaultSyncPut controllers.
func NewDefaultSyncPutFactory[T any](rootScope string, opts ctrl.ResourceOptions[T]) func(ctrl.Options) (ctrl.Controller, error) {
	return func(ctrlOpts ctrl.Options) (ctrl.Controller, error) {
		return &DefaultSyncPut[T]{
			BaseController: ctrl.NewBaseController(ctrlOpts),
			opts:           opts,
			rootScope:      rootScope,
		}, nil
	}
}

func (c *DefaultSyncPut[T]) Run(ctx context.Context, w http.ResponseWriter, req *http.Request) (v1.Response, error) {
	name := chi.URLParam(req, "name")
	if name == "" {
		return v1.NewBadRequestResponse("name is required"), nil
	}

	body, err := io.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		return v1.NewBadRequestResponse("failed to read request body"), nil
	}

	newResource, err := c.opts.RequestConverter(body)
	if err != nil {
		return v1.NewBadRequestResponse("invalid request body: " + err.Error()), nil
	}

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
		opts := &ctrl.Options{
			Address:        c.Options().Address,
			DatabaseClient: c.DatabaseClient(),
			StatusManager:  c.StatusManager(),
			ResourceType:   c.ResourceType(),
		}
		resp, err := filter(ctx, newResource, oldPtr, opts)
		if err != nil {
			return nil, err
		}
		if resp != nil {
			return resp, nil
		}
	}

	// Save resource
	saveOpts := ""
	if !isNew {
		saveOpts = etag
	}
	_, err = c.SaveResource(ctx, id, newResource, saveOpts)
	if err != nil {
		return nil, err
	}

	// Convert to response
	if c.opts.ResponseConverter != nil {
		resp, err := c.opts.ResponseConverter(newResource)
		if err != nil {
			return nil, err
		}
		if isNew {
			return v1.NewCreatedResponse(resp), nil
		}
		return v1.NewOKResponse(resp), nil
	}

	if isNew {
		return v1.NewCreatedResponse(newResource), nil
	}
	return v1.NewOKResponse(newResource), nil
}

// DefaultSyncDelete handles synchronous DELETE operations with typed filters.
type DefaultSyncDelete[T any] struct {
	ctrl.BaseController
	opts      ctrl.ResourceOptions[T]
	rootScope string
}

// NewDefaultSyncDeleteFactory creates a factory for DefaultSyncDelete controllers.
func NewDefaultSyncDeleteFactory[T any](rootScope string, opts ctrl.ResourceOptions[T]) func(ctrl.Options) (ctrl.Controller, error) {
	return func(ctrlOpts ctrl.Options) (ctrl.Controller, error) {
		return &DefaultSyncDelete[T]{
			BaseController: ctrl.NewBaseController(ctrlOpts),
			opts:           opts,
			rootScope:      rootScope,
		}, nil
	}
}

func (c *DefaultSyncDelete[T]) Run(ctx context.Context, w http.ResponseWriter, req *http.Request) (v1.Response, error) {
	name := chi.URLParam(req, "name")
	if name == "" {
		return v1.NewBadRequestResponse("name is required"), nil
	}

	id := buildResourceID(c.rootScope, c.ResourceType(), name, req)

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

	err = c.DatabaseClient().Delete(ctx, id)
	if err != nil {
		if errors.Is(err, &database.ErrNotFound{}) {
			return v1.NewNoContentResponse(), nil
		}
		return nil, err
	}

	return v1.NewNoContentResponse(), nil
}
