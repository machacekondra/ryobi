package grpcapi

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/components/database"
	"github.com/ryobi-project/ryobi/pkg/placement"
	"github.com/ryobi-project/ryobi/pkg/resources/datamodel"
)

// EnvironmentServer implements the gRPC EnvironmentService.
// It manages connected environment agents and dispatches resource events to them.
type EnvironmentServer struct {
	UnimplementedEnvironmentServiceServer

	mu           sync.RWMutex
	watchers     map[string][]chan *ResourceEvent    // environment name -> event channels
	environments map[string]*placement.EnvironmentInfo // environment name -> capabilities
	db           database.Client
	sm           ctrl.StatusManager
	logger       logr.Logger
}

// NewEnvironmentServer creates a new EnvironmentServer.
func NewEnvironmentServer(db database.Client, sm ctrl.StatusManager, logger logr.Logger) *EnvironmentServer {
	return &EnvironmentServer{
		watchers:     make(map[string][]chan *ResourceEvent),
		environments: make(map[string]*placement.EnvironmentInfo),
		db:           db,
		sm:           sm,
		logger:       logger,
	}
}

// GetEnvironments returns a snapshot of all registered environments for placement decisions.
func (s *EnvironmentServer) GetEnvironments() []placement.EnvironmentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]placement.EnvironmentInfo, 0, len(s.environments))
	for _, env := range s.environments {
		info := *env
		// Mark as connected if there are active watchers
		_, hasWatchers := s.watchers[info.Name]
		info.Connected = hasWatchers && len(s.watchers[info.Name]) > 0
		result = append(result, info)
	}
	return result
}

// Register handles environment agent registration.
func (s *EnvironmentServer) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	s.logger.Info("Environment agent registering", "environment", req.EnvironmentName)

	// Build environment data model from the registration
	env := &datamodel.Environment{
		Name: req.EnvironmentName,
		Type: datamodel.EnvironmentResourceType,
		Properties: datamodel.EnvironmentProperties{
			Providers: make(map[string]datamodel.ProviderConfig),
			Recipes:   make(map[string]map[string]datamodel.RecipeConfig),
		},
	}

	// Map providers
	for name, p := range req.Config.Providers {
		env.Properties.Providers[name] = datamodel.ProviderConfig{Scope: p.Scope}
	}

	// Map terraform provider configs
	if len(req.Config.TerraformProviders) > 0 {
		tfConfig := &datamodel.TerraformConfig{
			Providers: make(map[string]map[string]any),
		}
		for name, jsonConfig := range req.Config.TerraformProviders {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(jsonConfig), &parsed); err != nil {
				tfConfig.Providers[name] = map[string]any{"raw": jsonConfig}
			} else {
				tfConfig.Providers[name] = parsed
			}
		}
		env.Properties.RecipeConfig = &datamodel.RecipeGlobalConfig{Terraform: tfConfig}
	}

	// Map recipes
	for _, recipe := range req.Recipes {
		if env.Properties.Recipes[recipe.ResourceType] == nil {
			env.Properties.Recipes[recipe.ResourceType] = make(map[string]datamodel.RecipeConfig)
		}
		params := make(map[string]any)
		for k, v := range recipe.DefaultParameters {
			params[k] = v
		}
		env.Properties.Recipes[recipe.ResourceType][recipe.RecipeName] = datamodel.RecipeConfig{
			TemplateKind: "terraform",
			TemplatePath: recipe.TemplatePath,
			Parameters:   params,
		}
	}

	// Save environment to database
	envID := "/api/v1/ryobi/environments/" + req.EnvironmentName
	env.ID = envID
	obj := &database.Object{
		Metadata: database.Metadata{
			ID:           envID,
			ResourceType: datamodel.EnvironmentResourceType,
			RootScope:    "/api/v1",
		},
		Data: env,
	}
	if err := s.db.Save(ctx, obj); err != nil {
		return &RegisterResponse{Success: false, Message: err.Error()}, nil
	}

	// Store placement capabilities
	envInfo := &placement.EnvironmentInfo{
		Name:       req.EnvironmentName,
		RecipeTypes: make(map[string]string),
	}
	if req.Capabilities != nil {
		envInfo.Static = placement.StaticCapabilities{
			Region:       req.Capabilities.Region,
			Sovereignty:  req.Capabilities.Sovereignty,
			Capabilities: req.Capabilities.Capabilities,
			CostPerHour:  req.Capabilities.CostPerHour,
			MaxReplicas:  req.Capabilities.MaxReplicas,
		}
	}
	for _, recipe := range req.Recipes {
		envInfo.RecipeTypes[recipe.ResourceType] = recipe.RecipeName
	}

	s.mu.Lock()
	s.environments[req.EnvironmentName] = envInfo
	s.mu.Unlock()

	s.logger.Info("Environment registered", "environment", req.EnvironmentName, "recipes", len(req.Recipes))

	return &RegisterResponse{
		Success:       true,
		Message:       "environment registered",
		EnvironmentId: envID,
	}, nil
}

// Unregister removes an environment agent.
func (s *EnvironmentServer) Unregister(ctx context.Context, req *UnregisterRequest) (*UnregisterResponse, error) {
	s.logger.Info("Environment agent unregistering", "environment", req.EnvironmentName)

	s.mu.Lock()
	// Close all watcher channels for this environment
	if channels, ok := s.watchers[req.EnvironmentName]; ok {
		for _, ch := range channels {
			close(ch)
		}
		delete(s.watchers, req.EnvironmentName)
	}
	delete(s.environments, req.EnvironmentName)
	s.mu.Unlock()

	// Remove environment from database
	envID := "/api/v1/ryobi/environments/" + req.EnvironmentName
	if err := s.db.Delete(ctx, envID); err != nil {
		s.logger.Error(err, "Failed to delete environment from database", "environment", req.EnvironmentName)
	} else {
		s.logger.Info("Environment removed from database", "environment", req.EnvironmentName)
	}

	return &UnregisterResponse{Success: true}, nil
}

// WatchResources streams resource events to the environment agent.
func (s *EnvironmentServer) WatchResources(req *WatchRequest, stream EnvironmentService_WatchResourcesServer) error {
	s.logger.Info("Environment agent watching", "environment", req.EnvironmentName)

	ch := make(chan *ResourceEvent, 64)

	s.mu.Lock()
	s.watchers[req.EnvironmentName] = append(s.watchers[req.EnvironmentName], ch)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		channels := s.watchers[req.EnvironmentName]
		for i, c := range channels {
			if c == ch {
				s.watchers[req.EnvironmentName] = append(channels[:i], channels[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
	}()

	for {
		select {
		case <-stream.Context().Done():
			s.logger.Info("Watch stream closed", "environment", req.EnvironmentName)
			return nil
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(event); err != nil {
				s.logger.Error(err, "Failed to send event", "environment", req.EnvironmentName)
				return err
			}
		}
	}
}

// ReportResult handles the result of a resource operation from the environment agent.
func (s *EnvironmentServer) ReportResult(ctx context.Context, req *ReportResultRequest) (*ReportResultResponse, error) {
	s.logger.Info("Received operation result",
		"operationId", req.OperationId,
		"resourceId", req.ResourceId,
		"success", req.Success)

	// Update resource status in database
	obj, err := s.db.Get(ctx, req.ResourceId)
	if err != nil {
		s.logger.Error(err, "Failed to get resource for result update", "resourceId", req.ResourceId)
		return &ReportResultResponse{Acknowledged: true}, nil
	}

	var resource datamodel.Resource
	if err := obj.As(&resource); err != nil {
		s.logger.Error(err, "Failed to deserialize resource", "resourceId", req.ResourceId)
		return &ReportResultResponse{Acknowledged: true}, nil
	}

	if req.Success {
		resource.Properties.Status.State = datamodel.StateSucceeded
		resource.Properties.Status.Error = ""
		if len(req.Outputs) > 0 {
			resource.Properties.Status.Outputs = make(map[string]any)
			for k, v := range req.Outputs {
				resource.Properties.Status.Outputs[k] = v
			}
		}
	} else {
		resource.Properties.Status.State = datamodel.StateFailed
		resource.Properties.Status.Error = req.ErrorMessage
	}

	obj.Data = &resource
	if err := s.db.Save(ctx, obj, database.WithETag(obj.ETag)); err != nil {
		s.logger.Error(err, "Failed to save resource status", "resourceId", req.ResourceId)
	}

	// Update operation status
	state := v1.ProvisioningStateSucceeded
	var result *ctrl.OperationResult
	if !req.Success {
		state = v1.ProvisioningStateFailed
		result = &ctrl.OperationResult{
			Error: &v1.ErrorDetails{
				Code:    "RecipeExecutionFailed",
				Message: req.ErrorMessage,
			},
		}
	}
	if err := s.sm.Update(ctx, req.OperationId, state, result); err != nil {
		s.logger.Error(err, "Failed to update operation status", "operationId", req.OperationId)
	}

	return &ReportResultResponse{Acknowledged: true}, nil
}

// DispatchResourceEvent sends a resource event to all watchers for the given environment.
// Called by ryobid when a resource PUT/DELETE is received.
func (s *EnvironmentServer) DispatchResourceEvent(environmentName string, event *ResourceEvent) error {
	s.mu.RLock()
	channels, ok := s.watchers[environmentName]
	s.mu.RUnlock()

	if !ok || len(channels) == 0 {
		return fmt.Errorf("no environment agent connected for environment %q", environmentName)
	}

	for _, ch := range channels {
		select {
		case ch <- event:
		default:
			s.logger.Info("Warning: event channel full, dropping event",
				"environment", environmentName,
				"operationId", event.OperationId)
		}
	}

	return nil
}

// Heartbeat updates dynamic capabilities for an environment.
func (s *EnvironmentServer) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	env, ok := s.environments[req.EnvironmentName]
	if !ok {
		return &HeartbeatResponse{Acknowledged: false}, nil
	}

	env.Dynamic = placement.DynamicCapabilities{
		AvailableCPUMillicores: req.AvailableCpuMillicores,
		AvailableMemoryMB:      req.AvailableMemoryMb,
		RunningResources:       req.RunningResources,
		LastHeartbeat:          time.Now().Unix(),
	}

	return &HeartbeatResponse{Acknowledged: true}, nil
}
