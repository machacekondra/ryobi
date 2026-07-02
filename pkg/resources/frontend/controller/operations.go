package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/components/database"
)

// GetOperationStatus handles GET /api/v1/operations/{operationId}.
type GetOperationStatus struct {
	ctrl.BaseController
}

// NewGetOperationStatus creates a new GetOperationStatus controller.
func NewGetOperationStatus(opts ctrl.Options) (ctrl.Controller, error) {
	return &GetOperationStatus{
		BaseController: ctrl.NewBaseController(opts),
	}, nil
}

func (c *GetOperationStatus) Run(ctx context.Context, w http.ResponseWriter, req *http.Request) (v1.Response, error) {
	operationID := chi.URLParam(req, "operationId")
	if operationID == "" {
		return v1.NewBadRequestResponse("operationId is required"), nil
	}

	status, err := c.StatusManager().Get(ctx, operationID)
	if err != nil {
		if errors.Is(err, &database.ErrNotFound{}) {
			return v1.NewNotFoundResponse(operationID), nil
		}
		return nil, err
	}

	return v1.NewOKResponse(status), nil
}

// GenericGet handles GET for any resource by ID.
type GenericGet struct {
	ctrl.BaseController
	rootScope string
}

// NewGenericGetFactory creates a factory for GenericGet controllers.
func NewGenericGetFactory(rootScope string) func(ctrl.Options) (ctrl.Controller, error) {
	return func(opts ctrl.Options) (ctrl.Controller, error) {
		return &GenericGet{
			BaseController: ctrl.NewBaseController(opts),
			rootScope:      rootScope,
		}, nil
	}
}

func (c *GenericGet) Run(ctx context.Context, w http.ResponseWriter, req *http.Request) (v1.Response, error) {
	name := chi.URLParam(req, "name")
	if name == "" {
		return v1.NewBadRequestResponse("name is required"), nil
	}

	id := buildID(c.rootScope, c.ResourceType(), name, req)

	var resource map[string]any
	_, err := c.GetResource(ctx, id, &resource)
	if err != nil {
		if errors.Is(err, &database.ErrNotFound{}) {
			return v1.NewNotFoundResponse(id), nil
		}
		return nil, err
	}

	return v1.NewOKResponse(resource), nil
}

// GenericList handles LIST for any resource type.
type GenericList struct {
	ctrl.BaseController
	rootScope string
}

// NewGenericListFactory creates a factory for GenericList controllers.
func NewGenericListFactory(rootScope string) func(ctrl.Options) (ctrl.Controller, error) {
	return func(opts ctrl.Options) (ctrl.Controller, error) {
		return &GenericList{
			BaseController: ctrl.NewBaseController(opts),
			rootScope:      rootScope,
		}, nil
	}
}

func (c *GenericList) Run(ctx context.Context, w http.ResponseWriter, req *http.Request) (v1.Response, error) {
	query := database.Query{
		RootScope:    c.rootScope,
		ResourceType: c.ResourceType(),
	}

	// If scoped to an application, extract it from the URL
	appName := chi.URLParam(req, "appName")
	if appName != "" {
		query.RootScope = c.rootScope + "/ryobi/applications/" + appName
	}

	result, err := c.DatabaseClient().Query(ctx, query)
	if err != nil {
		return nil, err
	}

	items := make([]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, item.Data)
	}

	return v1.NewOKResponse(map[string]any{"value": items}), nil
}

// GenericPut handles PUT for any resource (sync).
type GenericPut struct {
	ctrl.BaseController
	rootScope string
}

// NewGenericPutFactory creates a factory for sync GenericPut controllers.
func NewGenericPutFactory(rootScope string) func(ctrl.Options) (ctrl.Controller, error) {
	return func(opts ctrl.Options) (ctrl.Controller, error) {
		return &GenericPut{
			BaseController: ctrl.NewBaseController(opts),
			rootScope:      rootScope,
		}, nil
	}
}

func (c *GenericPut) Run(ctx context.Context, w http.ResponseWriter, req *http.Request) (v1.Response, error) {
	name := chi.URLParam(req, "name")
	if name == "" {
		return v1.NewBadRequestResponse("name is required"), nil
	}

	var body map[string]any
	if err := ctrl.ReadJSONBody(req, &body); err != nil {
		return v1.NewBadRequestResponse("invalid request body: " + err.Error()), nil
	}

	id := buildID(c.rootScope, c.ResourceType(), name, req)
	rootScope := buildRootScope(c.rootScope, req)
	body["id"] = id
	body["name"] = name
	body["type"] = c.ResourceType()

	obj := &database.Object{
		Metadata: database.Metadata{
			ID:           id,
			ResourceType: c.ResourceType(),
			RootScope:    rootScope,
		},
		Data: body,
	}

	if err := c.DatabaseClient().Save(ctx, obj); err != nil {
		return nil, err
	}

	return v1.NewOKResponse(body), nil
}

// GenericDelete handles DELETE for any resource (sync).
type GenericDelete struct {
	ctrl.BaseController
	rootScope string
}

// NewGenericDeleteFactory creates a factory for GenericDelete controllers.
func NewGenericDeleteFactory(rootScope string) func(ctrl.Options) (ctrl.Controller, error) {
	return func(opts ctrl.Options) (ctrl.Controller, error) {
		return &GenericDelete{
			BaseController: ctrl.NewBaseController(opts),
			rootScope:      rootScope,
		}, nil
	}
}

func (c *GenericDelete) Run(ctx context.Context, w http.ResponseWriter, req *http.Request) (v1.Response, error) {
	name := chi.URLParam(req, "name")
	if name == "" {
		return v1.NewBadRequestResponse("name is required"), nil
	}

	id := buildID(c.rootScope, c.ResourceType(), name, req)

	err := c.DatabaseClient().Delete(ctx, id)
	if err != nil {
		if errors.Is(err, &database.ErrNotFound{}) {
			return v1.NewNoContentResponse(), nil
		}
		return nil, err
	}

	return v1.NewNoContentResponse(), nil
}

// buildID constructs a resource ID, including application scope when present.
func buildID(rootScope, resourceType, name string, req *http.Request) string {
	appName := chi.URLParam(req, "appName")
	if appName != "" {
		return rootScope + "/ryobi/applications/" + appName + "/" + resourceType + "/" + name
	}
	return rootScope + "/" + resourceType + "/" + name
}

// buildRootScope returns the root scope for queries, including application scope when present.
func buildRootScope(rootScope string, req *http.Request) string {
	appName := chi.URLParam(req, "appName")
	if appName != "" {
		return rootScope + "/ryobi/applications/" + appName
	}
	return rootScope
}
