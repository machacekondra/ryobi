package async

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/components/database"
	"github.com/ryobi-project/ryobi/pkg/components/queue"
)

const (
	operationStatusPrefix = "/api/v1/operations/"
	defaultTimeout        = 10 * time.Minute
)

// statusManager implements the StatusManager interface.
type statusManager struct {
	db      database.Client
	queue   queue.Client
	address string
}

// NewStatusManager creates a new StatusManager.
func NewStatusManager(db database.Client, q queue.Client, address string) ctrl.StatusManager {
	return &statusManager{
		db:      db,
		queue:   q,
		address: address,
	}
}

// AsyncOperationMessage is the message sent to the queue for async processing.
type AsyncOperationMessage struct {
	OperationID   string           `json:"operationId"`
	ResourceID    string           `json:"resourceId"`
	OperationType v1.OperationType `json:"operationType"`
}

func (sm *statusManager) QueueAsyncOperation(ctx context.Context, operationID string, resourceID string, operationType v1.OperationType, timeout time.Duration) (string, error) {
	if operationID == "" {
		operationID = uuid.New().String()
	}
	if timeout == 0 {
		timeout = defaultTimeout
	}

	status := &ctrl.OperationStatus{
		ID:            operationID,
		ResourceID:    resourceID,
		OperationType: operationType,
		State:         v1.ProvisioningStateAccepted,
		StartTime:     time.Now(),
	}

	obj := &database.Object{
		Metadata: database.Metadata{
			ID:           operationStatusPrefix + operationID,
			ResourceType: "operations",
			RootScope:    "/",
		},
		Data: status,
	}
	if err := sm.db.Save(ctx, obj); err != nil {
		return "", fmt.Errorf("failed to save operation status: %w", err)
	}

	msg := AsyncOperationMessage{
		OperationID:   operationID,
		ResourceID:    resourceID,
		OperationType: operationType,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal operation message: %w", err)
	}

	queueMsg := &queue.Message{
		Data: data,
	}
	if err := sm.queue.Enqueue(ctx, queueMsg); err != nil {
		return "", fmt.Errorf("failed to enqueue operation: %w", err)
	}

	operationURL := fmt.Sprintf("http://%s%s%s", sm.address, operationStatusPrefix, operationID)
	return operationURL, nil
}

func (sm *statusManager) Get(ctx context.Context, operationID string) (*ctrl.OperationStatus, error) {
	obj, err := sm.db.Get(ctx, operationStatusPrefix+operationID)
	if err != nil {
		return nil, err
	}

	var status ctrl.OperationStatus
	if err := obj.As(&status); err != nil {
		return nil, fmt.Errorf("failed to deserialize operation status: %w", err)
	}

	return &status, nil
}

func (sm *statusManager) Update(ctx context.Context, operationID string, state v1.ProvisioningState, result *ctrl.OperationResult) error {
	obj, err := sm.db.Get(ctx, operationStatusPrefix+operationID)
	if err != nil {
		return err
	}

	var status ctrl.OperationStatus
	if err := obj.As(&status); err != nil {
		return fmt.Errorf("failed to deserialize operation status: %w", err)
	}

	status.State = state
	if state.IsTerminal() {
		now := time.Now()
		status.EndTime = &now
	}
	if result != nil && result.Error != nil {
		status.Error = result.Error
	}

	obj.Data = &status
	return sm.db.Save(ctx, obj, database.WithETag(obj.ETag))
}

func (sm *statusManager) Delete(ctx context.Context, operationID string) error {
	return sm.db.Delete(ctx, operationStatusPrefix+operationID)
}
