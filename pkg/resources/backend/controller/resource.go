package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/api/async"
	"github.com/ryobi-project/ryobi/pkg/components/database"
	"github.com/ryobi-project/ryobi/pkg/recipes"
	"github.com/ryobi-project/ryobi/pkg/resources/datamodel"
)

// DeployResource is the async controller for deploying a Terraform-managed resource.
type DeployResource struct {
	db     database.Client
	engine recipes.Engine
}

// NewDeployResource creates a new DeployResource controller.
func NewDeployResource(db database.Client, engine recipes.Engine) *DeployResource {
	return &DeployResource{db: db, engine: engine}
}

func (c *DeployResource) Run(ctx context.Context, request *async.AsyncRequest) (ctrl.OperationResult, error) {
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("Deploying resource", "resourceId", request.ResourceID)

	// Get the resource from the database
	obj, err := c.db.Get(ctx, request.ResourceID)
	if err != nil {
		return ctrl.OperationResult{}, fmt.Errorf("failed to get resource: %w", err)
	}

	var resource datamodel.Resource
	if err := obj.As(&resource); err != nil {
		return ctrl.OperationResult{}, fmt.Errorf("failed to deserialize resource: %w", err)
	}

	// Resolve the environment and recipe
	envConfig, envRecipe, err := c.resolveRecipe(ctx, &resource)
	if err != nil {
		return failedResult(err), nil
	}

	// Build resource metadata
	resourceMeta := &recipes.ResourceMetadata{
		Name:       resource.Name,
		ResourceID: request.ResourceID,
		Parameters: resource.Properties.Parameters,
	}

	// Execute the recipe
	output, err := c.engine.Execute(ctx, envConfig, envRecipe, resourceMeta)
	if err != nil {
		// Update resource status to failed
		resource.Properties.Status.State = datamodel.StateFailed
		obj.Data = &resource
		_ = c.db.Save(ctx, obj, database.WithETag(obj.ETag))
		return failedResult(err), nil
	}

	// Update resource status with outputs
	resource.Properties.Status.State = datamodel.StateSucceeded
	if output != nil {
		resource.Properties.Status.Outputs = output.Values
		resource.Properties.Status.OutputResources = output.Resources
	}
	obj.Data = &resource
	if err := c.db.Save(ctx, obj, database.WithETag(obj.ETag)); err != nil {
		return ctrl.OperationResult{}, fmt.Errorf("failed to save resource status: %w", err)
	}

	logger.Info("Resource deployed successfully", "resourceId", request.ResourceID)
	return ctrl.OperationResult{}, nil
}

// resolveRecipe looks up the environment, extracts recipe config, and finds the recipe definition.
func (c *DeployResource) resolveRecipe(ctx context.Context, resource *datamodel.Resource) (*recipes.Configuration, *recipes.EnvironmentDefinition, error) {
	// The resource is scoped to an application; find the application to get the environment
	// For now, we search for the application by walking the resource ID path
	// Resource IDs look like: /api/v1/ryobi/applications/{app}/ryobi/resources/{name}
	appID, err := extractApplicationID(resource.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to extract application ID: %w", err)
	}

	// Get application
	appObj, err := c.db.Get(ctx, appID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get application: %w", err)
	}
	var app datamodel.Application
	if err := appObj.As(&app); err != nil {
		return nil, nil, fmt.Errorf("failed to deserialize application: %w", err)
	}

	// Get environment
	envID := "/api/v1/ryobi/environments/" + app.Properties.Environment
	envObj, err := c.db.Get(ctx, envID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get environment '%s': %w", app.Properties.Environment, err)
	}
	var env datamodel.Environment
	if err := envObj.As(&env); err != nil {
		return nil, nil, fmt.Errorf("failed to deserialize environment: %w", err)
	}

	// Build configuration from environment
	envConfig := &recipes.Configuration{
		Providers: make(map[string]recipes.ProviderConfig),
	}
	for name, p := range env.Properties.Providers {
		envConfig.Providers[name] = recipes.ProviderConfig{Scope: p.Scope}
	}
	if env.Properties.RecipeConfig != nil && env.Properties.RecipeConfig.Terraform != nil {
		envConfig.RecipeConfig.Terraform.Providers = convertProviders(env.Properties.RecipeConfig.Terraform.Providers)
	}

	// Resolve recipe definition
	envRecipes := make(map[string]map[string]recipes.EnvironmentDefinition)
	for resourceType, recipeMap := range env.Properties.Recipes {
		envRecipes[resourceType] = make(map[string]recipes.EnvironmentDefinition)
		for recipeName, rc := range recipeMap {
			envRecipes[resourceType][recipeName] = recipes.EnvironmentDefinition{
				Name:         recipeName,
				ResourceType: resourceType,
				TemplatePath: rc.TemplatePath,
				Parameters:   rc.Parameters,
			}
		}
	}

	envRecipe, err := recipes.ResolveRecipe(envRecipes, resource.Properties.ResourceType, resource.Properties.RecipeName)
	if err != nil {
		return nil, nil, err
	}

	return envConfig, envRecipe, nil
}

// DeleteResource is the async controller for deleting a Terraform-managed resource.
type DeleteResource struct {
	db     database.Client
	engine recipes.Engine
}

// NewDeleteResource creates a new DeleteResource controller.
func NewDeleteResource(db database.Client, engine recipes.Engine) *DeleteResource {
	return &DeleteResource{db: db, engine: engine}
}

func (c *DeleteResource) Run(ctx context.Context, request *async.AsyncRequest) (ctrl.OperationResult, error) {
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("Deleting resource", "resourceId", request.ResourceID)

	// Get the resource
	obj, err := c.db.Get(ctx, request.ResourceID)
	if err != nil {
		if isNotFound(err) {
			logger.Info("Resource already deleted", "resourceId", request.ResourceID)
			return ctrl.OperationResult{}, nil
		}
		return ctrl.OperationResult{}, fmt.Errorf("failed to get resource: %w", err)
	}

	var resource datamodel.Resource
	if err := obj.As(&resource); err != nil {
		return ctrl.OperationResult{}, fmt.Errorf("failed to deserialize resource: %w", err)
	}

	// Resolve recipe for destruction
	envConfig, envRecipe, err := c.resolveRecipe(ctx, &resource)
	if err != nil {
		logger.Error(err, "failed to resolve recipe for deletion, deleting resource record only")
	} else {
		// Execute terraform destroy
		resourceMeta := &recipes.ResourceMetadata{
			Name:       resource.Name,
			ResourceID: request.ResourceID,
			Parameters: resource.Properties.Parameters,
		}
		if err := c.engine.Delete(ctx, envConfig, envRecipe, resourceMeta); err != nil {
			return failedResult(err), nil
		}
	}

	// Delete the resource from the database
	if err := c.db.Delete(ctx, request.ResourceID); err != nil && !isNotFound(err) {
		return ctrl.OperationResult{}, fmt.Errorf("failed to delete resource: %w", err)
	}

	logger.Info("Resource deleted", "resourceId", request.ResourceID)
	return ctrl.OperationResult{}, nil
}

// resolveRecipe is the same as DeployResource.resolveRecipe.
func (c *DeleteResource) resolveRecipe(ctx context.Context, resource *datamodel.Resource) (*recipes.Configuration, *recipes.EnvironmentDefinition, error) {
	deploy := &DeployResource{db: c.db}
	return deploy.resolveRecipe(ctx, resource)
}

// extractApplicationID extracts the application ID from a resource ID.
// Resource IDs are like: /api/v1/ryobi/applications/{app}/ryobi/resources/{name}
func extractApplicationID(resourceID string) (string, error) {
	// Find the application segment
	const appsPrefix = "/ryobi/applications/"
	idx := findSubstring(resourceID, appsPrefix)
	if idx == -1 {
		return "", fmt.Errorf("resource ID %q does not contain an application scope", resourceID)
	}

	// Find the end of the app name (next /)
	afterPrefix := idx + len(appsPrefix)
	end := afterPrefix
	for end < len(resourceID) && resourceID[end] != '/' {
		end++
	}

	return resourceID[:end], nil
}

func findSubstring(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func isNotFound(err error) bool {
	_, ok := err.(*database.ErrNotFound)
	return ok
}

func failedResult(err error) ctrl.OperationResult {
	return ctrl.OperationResult{
		Error: &v1.ErrorDetails{
			Code:    "RecipeExecutionFailed",
			Message: err.Error(),
		},
	}
}

func convertProviders(providers map[string]map[string]any) map[string][]map[string]any {
	result := make(map[string][]map[string]any)
	for name, config := range providers {
		result[name] = []map[string]any{config}
	}
	return result
}
