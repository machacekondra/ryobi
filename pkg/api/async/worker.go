package async

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/components/queue"
)

const (
	defaultPollInterval = 1 * time.Second
)

// AsyncController is the interface for controllers that handle async operations.
type AsyncController interface {
	Run(ctx context.Context, request *AsyncRequest) (ctrl.OperationResult, error)
}

// AsyncControllerFunc is a function that implements AsyncController.
type AsyncControllerFunc func(ctx context.Context, request *AsyncRequest) (ctrl.OperationResult, error)

func (f AsyncControllerFunc) Run(ctx context.Context, request *AsyncRequest) (ctrl.OperationResult, error) {
	return f(ctx, request)
}

// AsyncRequest contains the details of an async operation to process.
type AsyncRequest struct {
	OperationID   string
	ResourceID    string
	OperationType v1.OperationType
}

// ControllerRegistry maps operation types to async controllers.
type ControllerRegistry struct {
	controllers map[string]AsyncController
}

// NewControllerRegistry creates a new ControllerRegistry.
func NewControllerRegistry() *ControllerRegistry {
	return &ControllerRegistry{
		controllers: make(map[string]AsyncController),
	}
}

// Register registers an async controller for the given resource type and method.
func (r *ControllerRegistry) Register(resourceType string, method v1.OperationMethod, controller AsyncController) {
	key := fmt.Sprintf("%s|%s", resourceType, method)
	r.controllers[key] = controller
}

// Get returns the async controller for the given resource type and method.
func (r *ControllerRegistry) Get(resourceType string, method v1.OperationMethod) (AsyncController, bool) {
	key := fmt.Sprintf("%s|%s", resourceType, method)
	c, ok := r.controllers[key]
	return c, ok
}

// Worker polls the queue and processes async operations.
type Worker struct {
	queue         queue.Client
	registry      *ControllerRegistry
	statusManager ctrl.StatusManager
	pollInterval  time.Duration
}

// NewWorker creates a new Worker.
func NewWorker(q queue.Client, registry *ControllerRegistry, sm ctrl.StatusManager) *Worker {
	return &Worker{
		queue:         q,
		registry:      registry,
		statusManager: sm,
		pollInterval:  defaultPollInterval,
	}
}

// Name returns the service name.
func (w *Worker) Name() string {
	return "async-worker"
}

// Run starts the worker loop.
func (w *Worker) Run(ctx context.Context) error {
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("Starting async worker")

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Async worker shutting down")
			return ctx.Err()
		case <-ticker.C:
			if err := w.processNext(ctx); err != nil {
				logger.Error(err, "error processing async operation")
			}
		}
	}
}

func (w *Worker) processNext(ctx context.Context) error {
	logger := logr.FromContextOrDiscard(ctx)

	msg, err := w.queue.Dequeue(ctx)
	if err != nil {
		return fmt.Errorf("failed to dequeue: %w", err)
	}
	if msg == nil {
		return nil
	}

	var opMsg AsyncOperationMessage
	if err := json.Unmarshal(msg.Data, &opMsg); err != nil {
		_ = w.queue.Complete(ctx, msg)
		return fmt.Errorf("failed to unmarshal operation message: %w", err)
	}

	logger.Info("Processing async operation",
		"operationId", opMsg.OperationID,
		"resourceType", opMsg.OperationType.Type,
		"method", opMsg.OperationType.Method)

	// Update status to in-progress
	_ = w.statusManager.Update(ctx, opMsg.OperationID, v1.ProvisioningStateProvisioning, nil)

	controller, ok := w.registry.Get(opMsg.OperationType.Type, opMsg.OperationType.Method)
	if !ok {
		_ = w.statusManager.Update(ctx, opMsg.OperationID, v1.ProvisioningStateFailed, &ctrl.OperationResult{
			Error: &v1.ErrorDetails{
				Code:    v1.CodeInternal,
				Message: fmt.Sprintf("no async controller registered for %s %s", opMsg.OperationType.Type, opMsg.OperationType.Method),
			},
		})
		_ = w.queue.Complete(ctx, msg)
		return nil
	}

	request := &AsyncRequest{
		OperationID:   opMsg.OperationID,
		ResourceID:    opMsg.ResourceID,
		OperationType: opMsg.OperationType,
	}

	result, err := controller.Run(ctx, request)
	if err != nil {
		_ = w.statusManager.Update(ctx, opMsg.OperationID, v1.ProvisioningStateFailed, &ctrl.OperationResult{
			Error: &v1.ErrorDetails{
				Code:    v1.CodeInternal,
				Message: err.Error(),
			},
		})
	} else if result.Error != nil {
		_ = w.statusManager.Update(ctx, opMsg.OperationID, v1.ProvisioningStateFailed, &result)
	} else {
		_ = w.statusManager.Update(ctx, opMsg.OperationID, v1.ProvisioningStateSucceeded, nil)
	}

	_ = w.queue.Complete(ctx, msg)

	logger.Info("Completed async operation", "operationId", opMsg.OperationID)
	return nil
}
