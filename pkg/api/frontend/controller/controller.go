package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	"github.com/ryobi-project/ryobi/pkg/components/database"
)

// Options represents controller options.
type Options struct {
	Address        string
	DatabaseClient database.Client
	StatusManager  StatusManager
	ResourceType   string
}

// Validate validates the Options.
func (o Options) Validate() error {
	var err error
	if o.Address == "" {
		err = errors.Join(err, errors.New(".Address is required"))
	}
	if o.DatabaseClient == nil {
		err = errors.Join(err, errors.New(".DatabaseClient is required"))
	}
	if o.ResourceType == "" {
		err = errors.Join(err, errors.New(".ResourceType is required"))
	}
	if o.StatusManager == nil {
		err = errors.Join(err, errors.New(".StatusManager is required"))
	}
	return err
}

// StatusManager is an interface to manage async operation status.
type StatusManager interface {
	QueueAsyncOperation(ctx context.Context, operationID string, resourceID string, operationType v1.OperationType, timeout time.Duration) (string, error)
	Get(ctx context.Context, operationID string) (*OperationStatus, error)
	Update(ctx context.Context, operationID string, state v1.ProvisioningState, result *OperationResult) error
	Delete(ctx context.Context, operationID string) error
}

// OperationStatus represents the status of an async operation.
type OperationStatus struct {
	ID             string                `json:"id"`
	ResourceID     string                `json:"resourceId"`
	OperationType  v1.OperationType      `json:"operationType"`
	State          v1.ProvisioningState  `json:"status"`
	StartTime      time.Time             `json:"startTime"`
	EndTime        *time.Time            `json:"endTime,omitempty"`
	Error          *v1.ErrorDetails      `json:"error,omitempty"`
}

// OperationResult holds the outcome of an async operation.
type OperationResult struct {
	Error *v1.ErrorDetails `json:"error,omitempty"`
}

// Controller is the interface for operation controllers.
type Controller interface {
	Run(ctx context.Context, w http.ResponseWriter, req *http.Request) (v1.Response, error)
}

// ResourceOptions represents options and filters for typed resource operations.
type ResourceOptions[T any] struct {
	RequestConverter  func(body []byte) (*T, error)
	ResponseConverter func(resource *T) (any, error)
	DeleteFilters     []DeleteFilter[T]
	UpdateFilters     []UpdateFilter[T]
	AsyncOperationTimeout    time.Duration
	AsyncOperationRetryAfter time.Duration
}

// DeleteFilter is a function executed before deleting a resource.
type DeleteFilter[T any] func(ctx context.Context, oldResource *T, options *Options) (v1.Response, error)

// UpdateFilter is a function executed before updating a resource.
type UpdateFilter[T any] func(ctx context.Context, newResource *T, oldResource *T, options *Options) (v1.Response, error)

// BaseController provides common functionality for controllers.
type BaseController struct {
	options Options
}

// NewBaseController creates a new BaseController.
func NewBaseController(options Options) BaseController {
	return BaseController{options: options}
}

// DatabaseClient returns the database client.
func (b *BaseController) DatabaseClient() database.Client {
	return b.options.DatabaseClient
}

// ResourceType returns the resource type.
func (b *BaseController) ResourceType() string {
	return b.options.ResourceType
}

// StatusManager returns the status manager.
func (b *BaseController) StatusManager() StatusManager {
	return b.options.StatusManager
}

// Options returns the controller options.
func (b *BaseController) Options() Options {
	return b.options
}

// GetResource retrieves a resource from the data store.
func (b *BaseController) GetResource(ctx context.Context, id string, out any) (string, error) {
	res, err := b.DatabaseClient().Get(ctx, id)
	if err != nil {
		return "", err
	}
	if err = res.As(out); err != nil {
		return "", err
	}
	return res.ETag, nil
}

// SaveResource persists a resource to the data store.
func (b *BaseController) SaveResource(ctx context.Context, id string, in any, etag string) (*database.Object, error) {
	return b.SaveResourceWithMeta(ctx, id, b.options.ResourceType, "", in, etag)
}

// SaveResourceWithMeta persists a resource with explicit metadata.
func (b *BaseController) SaveResourceWithMeta(ctx context.Context, id, resourceType, rootScope string, in any, etag string) (*database.Object, error) {
	obj := &database.Object{
		Metadata: database.Metadata{
			ID:           id,
			ResourceType: resourceType,
			RootScope:    rootScope,
		},
		Data: in,
	}

	opts := []database.SaveOptions{}
	if etag != "" {
		opts = append(opts, database.WithETag(etag))
	}

	err := b.DatabaseClient().Save(ctx, obj, opts...)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// ReadJSONBody reads and unmarshals the request body.
func ReadJSONBody(req *http.Request, target any) error {
	defer req.Body.Close()
	return json.NewDecoder(req.Body).Decode(target)
}
