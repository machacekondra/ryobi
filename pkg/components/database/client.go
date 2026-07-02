package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

var jsonPropertyPattern = "[a-zA-Z$_][a-zA-Z0-9$_]*"
var fieldRegex = regexp.MustCompile(fmt.Sprintf(`^(%s)(\.%s)*$`, jsonPropertyPattern, jsonPropertyPattern))

// Client is the interface for persisting and querying resource data.
type Client interface {
	Query(ctx context.Context, query Query, options ...QueryOptions) (*ObjectQueryResult, error)
	Get(ctx context.Context, id string, options ...GetOptions) (*Object, error)
	Delete(ctx context.Context, id string, options ...DeleteOptions) error
	Save(ctx context.Context, obj *Object, options ...SaveOptions) error
}

// Query specifies the structure of a query.
type Query struct {
	RootScope      string
	ScopeRecursive bool
	ResourceType   string
	Filters        []QueryFilter
}

// Validate validates the Query.
func (q Query) Validate() error {
	var err error
	if q.RootScope == "" {
		err = errors.Join(err, &ErrInvalid{Message: "RootScope is required"})
	}
	if q.ResourceType == "" {
		err = errors.Join(err, &ErrInvalid{Message: "ResourceType is required"})
	}
	for _, filter := range q.Filters {
		err = errors.Join(err, filter.Validate())
	}
	return err
}

// QueryFilter filters a property in a resource entity.
type QueryFilter struct {
	Field string
	Value string
}

// Validate validates the QueryFilter.
func (f QueryFilter) Validate() error {
	var err error
	if f.Field == "" {
		err = errors.Join(err, &ErrInvalid{Message: fmt.Sprintf("Field is required in filter: %+v", f)})
	}
	if !fieldRegex.Match([]byte(f.Field)) {
		err = errors.Join(err, &ErrInvalid{Message: fmt.Sprintf("Field is invalid in filter: %+v", f)})
	}
	return err
}

// QueryOptions are options for the Query operation.
type QueryOptions struct{}

// GetOptions are options for the Get operation.
type GetOptions struct{}

// DeleteOptions are options for the Delete operation.
type DeleteOptions struct {
	ETag string
}

// SaveOptions are options for the Save operation.
type SaveOptions struct {
	ETag string
}

// WithETag returns a SaveOptions with the given ETag.
func WithETag(etag string) SaveOptions {
	return SaveOptions{ETag: etag}
}

// Object is the stored representation of a resource.
type Object struct {
	Metadata Metadata
	Data     any
	ETag     string
}

// As deserializes the stored Data into the given target.
func (o *Object) As(target any) error {
	b, err := json.Marshal(o.Data)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}

// Metadata holds metadata about a stored object.
type Metadata struct {
	ID           string
	ResourceType string
	RootScope    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ObjectQueryResult is the result of a Query operation.
type ObjectQueryResult struct {
	Items []Object
}

// Error types.

// ErrNotFound is returned when a resource is not found.
type ErrNotFound struct {
	ID string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("resource not found: %s", e.ID)
}

func (e *ErrNotFound) Is(target error) bool {
	_, ok := target.(*ErrNotFound)
	return ok
}

// ErrConcurrency is returned when an optimistic concurrency check fails.
type ErrConcurrency struct {
	ID string
}

func (e *ErrConcurrency) Error() string {
	return fmt.Sprintf("optimistic concurrency conflict for resource: %s", e.ID)
}

func (e *ErrConcurrency) Is(target error) bool {
	_, ok := target.(*ErrConcurrency)
	return ok
}

// ErrInvalid is returned when a request contains invalid data.
type ErrInvalid struct {
	Message string
}

func (e *ErrInvalid) Error() string {
	return fmt.Sprintf("invalid request: %s", e.Message)
}

func (e *ErrInvalid) Is(target error) bool {
	_, ok := target.(*ErrInvalid)
	return ok
}
