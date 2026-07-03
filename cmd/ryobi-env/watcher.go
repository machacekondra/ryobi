package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ryobi-project/ryobi/pkg/grpcapi"
)

// watchedResource tracks a deployed resource for status monitoring.
type watchedResource struct {
	ResourceID   string
	ResourceName string
	ResourceType string
	// Terraform outputs used to find the K8s resource
	DeploymentName string
	Namespace      string
	// Last reported health state (for debouncing)
	lastState string
}

// statusWatcher monitors deployed resources and reports health changes.
type statusWatcher struct {
	mu        sync.Mutex
	resources map[string]*watchedResource // resourceID → watchedResource
	client    grpcapi.EnvironmentServiceClient
	k8s       kubernetes.Interface
	logger    logr.Logger
	interval  time.Duration
}

// newStatusWatcher creates a new status watcher.
func newStatusWatcher(client grpcapi.EnvironmentServiceClient, cfg *EnvConfig, logger logr.Logger) *statusWatcher {
	sw := &statusWatcher{
		resources: make(map[string]*watchedResource),
		client:    client,
		logger:    logger.WithName("status-watcher"),
		interval:  15 * time.Second,
	}

	// Try to create a Kubernetes client from the terraform provider config
	if kubeCfg, ok := cfg.TerraformProviders["kubernetes"]; ok {
		if configPath, ok := kubeCfg["config_path"].(string); ok {
			// Expand ~ to home directory
			if len(configPath) > 1 && configPath[:2] == "~/" {
				if home, err := os.UserHomeDir(); err == nil {
					configPath = home + configPath[1:]
				}
			}
			config, err := clientcmd.BuildConfigFromFlags("", configPath)
			if err != nil {
				logger.Error(err, "Failed to build kubeconfig", "path", configPath)
			} else {
				k8sClient, err := kubernetes.NewForConfig(config)
				if err != nil {
					logger.Error(err, "Failed to create Kubernetes client")
				} else {
					sw.k8s = k8sClient
					logger.Info("Kubernetes client initialized for status watching", "kubeconfig", configPath)
				}
			}
		} else {
			logger.Info("No config_path in kubernetes terraform provider, status watching disabled")
		}
	} else {
		logger.Info("No kubernetes terraform provider configured, status watching disabled")
	}

	return sw
}

// Register adds a resource to watch after successful deployment.
func (sw *statusWatcher) Register(resourceID, resourceName, resourceType string, params map[string]string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	wr := &watchedResource{
		ResourceID:   resourceID,
		ResourceName: resourceName,
		ResourceType: resourceType,
	}

	// Extract deployment name and namespace from parameters
	if name, ok := params["name"]; ok {
		wr.DeploymentName = unquote(name)
	} else {
		wr.DeploymentName = resourceName
	}
	if ns, ok := params["namespace"]; ok {
		wr.Namespace = unquote(ns)
	} else {
		wr.Namespace = "default"
	}

	sw.resources[resourceID] = wr
	sw.logger.Info("Registered resource for status watching",
		"resource", resourceName, "deployment", wr.DeploymentName, "namespace", wr.Namespace)
}

// Unregister removes a resource from watching.
func (sw *statusWatcher) Unregister(resourceID string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	delete(sw.resources, resourceID)
	sw.logger.Info("Unregistered resource from status watching", "resourceId", resourceID)
}

// Run starts the status watching loop.
func (sw *statusWatcher) Run(ctx context.Context) {
	sw.logger.Info("Starting status watcher", "interval", sw.interval)
	ticker := time.NewTicker(sw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			sw.logger.Info("Status watcher stopped")
			return
		case <-ticker.C:
			sw.checkAll(ctx)
		}
	}
}

func (sw *statusWatcher) checkAll(ctx context.Context) {
	sw.mu.Lock()
	// Copy the list to avoid holding the lock during API calls
	resources := make([]*watchedResource, 0, len(sw.resources))
	for _, wr := range sw.resources {
		resources = append(resources, wr)
	}
	sw.mu.Unlock()

	for _, wr := range resources {
		health := sw.checkResource(ctx, wr)
		if health == nil {
			continue
		}

		// Debounce: only report if state changed
		stateKey := fmt.Sprintf("%s:%d/%d", health.State, health.ReadyReplicas, health.DesiredReplicas)
		if stateKey == wr.lastState {
			continue
		}
		wr.lastState = stateKey

		sw.logger.Info("Resource health changed",
			"resource", wr.ResourceName,
			"state", health.State,
			"ready", fmt.Sprintf("%d/%d", health.ReadyReplicas, health.DesiredReplicas))

		_, err := sw.client.ReportStatus(ctx, &grpcapi.ReportStatusRequest{
			ResourceId:   wr.ResourceID,
			ResourceName: wr.ResourceName,
			Health:       health,
		})
		if err != nil {
			sw.logger.Error(err, "Failed to report status", "resource", wr.ResourceName)
		}
	}
}

func (sw *statusWatcher) checkResource(ctx context.Context, wr *watchedResource) *grpcapi.ResourceHealthStatus {
	switch wr.ResourceType {
	case "Ryobi.Compute/containers":
		return sw.checkDeployment(ctx, wr)
	default:
		return nil
	}
}

func (sw *statusWatcher) checkDeployment(ctx context.Context, wr *watchedResource) *grpcapi.ResourceHealthStatus {
	if sw.k8s == nil {
		return nil
	}

	deployment, err := sw.k8s.AppsV1().Deployments(wr.Namespace).Get(ctx, wr.DeploymentName, v1.GetOptions{})
	if err != nil {
		return &grpcapi.ResourceHealthStatus{
			State:       "Down",
			Message:     fmt.Sprintf("deployment not found: %v", err),
			LastUpdated: time.Now().Unix(),
		}
	}

	desired := deployment.Spec.Replicas
	if desired == nil {
		one := int32(1)
		desired = &one
	}
	ready := deployment.Status.ReadyReplicas

	state := "Running"
	message := fmt.Sprintf("%d/%d replicas ready", ready, *desired)

	if ready == 0 && *desired > 0 {
		state = "Down"
		// Check for specific conditions
		for _, cond := range deployment.Status.Conditions {
			if cond.Type == "Available" && cond.Status == "False" {
				message = cond.Message
			}
		}
	} else if ready < *desired {
		state = "Degraded"
	}

	properties := map[string]string{
		"availableReplicas":   fmt.Sprintf("%d", deployment.Status.AvailableReplicas),
		"updatedReplicas":     fmt.Sprintf("%d", deployment.Status.UpdatedReplicas),
		"observedGeneration":  fmt.Sprintf("%d", deployment.Status.ObservedGeneration),
	}

	return &grpcapi.ResourceHealthStatus{
		State:           state,
		Message:         message,
		ReadyReplicas:   ready,
		DesiredReplicas: *desired,
		LastUpdated:     time.Now().Unix(),
		Properties:      properties,
	}
}

// unquote removes JSON quotes from a string value.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
