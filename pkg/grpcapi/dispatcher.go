package grpcapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"

	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/api/async"
	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	"github.com/ryobi-project/ryobi/pkg/components/database"
	"github.com/ryobi-project/ryobi/pkg/placement"
	"github.com/ryobi-project/ryobi/pkg/resources/datamodel"
)

// ResourceDispatcher is an async controller that dispatches resource operations
// to connected environment agents via gRPC instead of executing Terraform directly.
type ResourceDispatcher struct {
	db              database.Client
	server          *EnvironmentServer
	placementEngine placement.Engine
}

// NewResourceDispatcher creates a new ResourceDispatcher.
func NewResourceDispatcher(db database.Client, server *EnvironmentServer, placementEngine placement.Engine) *ResourceDispatcher {
	return &ResourceDispatcher{db: db, server: server, placementEngine: placementEngine}
}

func (d *ResourceDispatcher) Run(ctx context.Context, request *async.AsyncRequest) (ctrl.OperationResult, error) {
	logger := logr.FromContextOrDiscard(ctx)

	// Get the resource
	obj, err := d.db.Get(ctx, request.ResourceID)
	if err != nil {
		return ctrl.OperationResult{}, fmt.Errorf("failed to get resource: %w", err)
	}

	var resource datamodel.Resource
	if err := obj.As(&resource); err != nil {
		return ctrl.OperationResult{}, fmt.Errorf("failed to deserialize resource: %w", err)
	}
	resource.ID = obj.Metadata.ID

	// Find the application to get the environment name
	appID, err := extractApplicationID(resource.ID)
	if err != nil {
		setResourceFailed(d.db, ctx, obj, &resource, "failed to extract application: "+err.Error())
		return failedResult(err), nil
	}

	appObj, err := d.db.Get(ctx, appID)
	if err != nil {
		setResourceFailed(d.db, ctx, obj, &resource, "failed to get application: "+err.Error())
		return failedResult(err), nil
	}

	var app datamodel.Application
	if err := appObj.As(&app); err != nil {
		setResourceFailed(d.db, ctx, obj, &resource, "failed to deserialize application: "+err.Error())
		return failedResult(err), nil
	}

	environmentName := app.Properties.Environment
	recipeName := resource.Properties.RecipeName

	// If no explicit environment or recipe, use placement engine
	if environmentName == "" || recipeName == "" {
		placementReq := placement.PlacementRequest{
			ResourceType: resource.Properties.ResourceType,
		}
		if resource.Properties.Placement != nil {
			placementReq.Constraints = resource.Properties.Placement.Constraints
			placementReq.Preferences = resource.Properties.Placement.Preferences
		}

		envs := d.server.GetEnvironments()
		result, err := d.placementEngine.Place(placementReq, envs)
		if err != nil {
			setResourceFailed(d.db, ctx, obj, &resource, "placement failed: "+err.Error())
			return failedResult(err), nil
		}

		if environmentName == "" {
			environmentName = result.EnvironmentName
		}
		if recipeName == "" {
			recipeName = result.RecipeName
		}

		logger.Info("Placement engine selected environment",
			"environment", environmentName,
			"recipe", recipeName,
			"resource", resource.Name)
	}

	// Convert parameters to string map for protobuf
	params := make(map[string]string)
	if resource.Properties.Parameters != nil {
		for k, v := range resource.Properties.Parameters {
			b, _ := json.Marshal(v)
			params[k] = string(b)
		}
	}

	// Determine operation type
	opType := OperationType_DEPLOY
	if request.OperationType.Method == v1.OperationDelete {
		opType = OperationType_DELETE
	}

	// Build the event
	event := &ResourceEvent{
		OperationId:     request.OperationID,
		ResourceId:      request.ResourceID,
		ResourceName:    resource.Name,
		ResourceType:    resource.Properties.ResourceType,
		RecipeName:      recipeName,
		Operation:       opType,
		Parameters:      params,
		ApplicationName: app.Name,
	}

	// Update resource status to deploying
	if opType == OperationType_DEPLOY {
		resource.Properties.Status.State = datamodel.StateDeploying
	} else {
		resource.Properties.Status.State = datamodel.StateDeleting
	}
	resource.Properties.Status.Error = ""
	obj.Data = &resource
	_ = d.db.Save(ctx, obj, database.WithETag(obj.ETag))

	// Dispatch to environment agent
	logger.Info("Dispatching resource event to environment agent",
		"environment", environmentName,
		"resource", resource.Name,
		"operation", opType.String())

	if err := d.server.DispatchResourceEvent(environmentName, event); err != nil {
		setResourceFailed(d.db, ctx, obj, &resource, "no environment agent connected: "+err.Error())
		return failedResult(err), nil
	}

	// The result will come back via ReportResult gRPC call.
	// Return empty result (no error) — the operation stays in "Provisioning" state
	// until the environment agent calls ReportResult.
	return ctrl.OperationResult{}, nil
}

func extractApplicationID(resourceID string) (string, error) {
	const appsPrefix = "/ryobi/applications/"
	idx := -1
	for i := 0; i <= len(resourceID)-len(appsPrefix); i++ {
		if resourceID[i:i+len(appsPrefix)] == appsPrefix {
			idx = i
			break
		}
	}
	if idx == -1 {
		return "", fmt.Errorf("resource ID %q does not contain an application scope", resourceID)
	}
	afterPrefix := idx + len(appsPrefix)
	end := afterPrefix
	for end < len(resourceID) && resourceID[end] != '/' {
		end++
	}
	return resourceID[:end], nil
}

func setResourceFailed(db database.Client, ctx context.Context, obj *database.Object, resource *datamodel.Resource, errMsg string) {
	resource.Properties.Status.State = datamodel.StateFailed
	resource.Properties.Status.Error = errMsg
	obj.Data = resource
	_ = db.Save(ctx, obj, database.WithETag(obj.ETag))
}

func failedResult(err error) ctrl.OperationResult {
	return ctrl.OperationResult{
		Error: &v1.ErrorDetails{
			Code:    "DispatchFailed",
			Message: err.Error(),
		},
	}
}
