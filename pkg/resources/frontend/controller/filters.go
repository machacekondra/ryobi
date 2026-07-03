package controller

import (
	"context"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/components/database"
	"github.com/ryobi-project/ryobi/pkg/resources/datamodel"
)

// ValidateEnvironmentUpdate validates that environment properties are well-formed.
func ValidateEnvironmentUpdate(ctx context.Context, newResource *datamodel.Environment, oldResource *datamodel.Environment, options *ctrl.Options) (v1.Response, error) {
	if newResource.Name == "" {
		return v1.NewBadRequestResponse("environment name is required"), nil
	}
	return nil, nil
}

// ValidateApplicationUpdate validates application properties.
// Note: environment existence is not checked here because environments are
// registered dynamically by agents and may not exist at app creation time.
// The environment is validated at dispatch time when resources are deployed.
func ValidateApplicationUpdate(ctx context.Context, newResource *datamodel.Application, oldResource *datamodel.Application, options *ctrl.Options) (v1.Response, error) {
	if newResource.Name == "" {
		return v1.NewBadRequestResponse("application name is required"), nil
	}
	return nil, nil
}

// PreventEnvironmentDeleteIfInUse prevents deletion of environments that have applications referencing them.
func PreventEnvironmentDeleteIfInUse(ctx context.Context, env *datamodel.Environment, options *ctrl.Options) (v1.Response, error) {
	// Query all applications
	result, err := options.DatabaseClient.Query(ctx, database.Query{
		RootScope:    "/api/v1",
		ResourceType: datamodel.ApplicationResourceType,
	})
	if err != nil {
		return nil, err
	}

	for _, item := range result.Items {
		var app datamodel.Application
		if err := item.As(&app); err != nil {
			continue
		}
		if app.Properties.Environment == env.Name {
			return v1.NewConflictResponse("cannot delete environment '" + env.Name + "': it is referenced by application '" + app.Name + "'"), nil
		}
	}

	return nil, nil
}

// ValidateResourceUpdate validates resource properties.
func ValidateResourceUpdate(ctx context.Context, newResource *datamodel.Resource, oldResource *datamodel.Resource, options *ctrl.Options) (v1.Response, error) {
	if newResource.Name == "" {
		return v1.NewBadRequestResponse("resource name is required"), nil
	}
	if newResource.Properties.ResourceType == "" {
		return v1.NewBadRequestResponse("resource type is required"), nil
	}
	// recipeName is optional — placement engine selects if omitted
	return nil, nil
}

func isNotFound(err error) bool {
	_, ok := err.(*database.ErrNotFound)
	return ok
}
