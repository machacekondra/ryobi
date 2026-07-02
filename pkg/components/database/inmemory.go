package database

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryClient is an in-memory implementation of the database Client for testing.
type InMemoryClient struct {
	mu    sync.RWMutex
	store map[string]*Object
}

// NewInMemoryClient creates a new InMemoryClient.
func NewInMemoryClient() *InMemoryClient {
	return &InMemoryClient{
		store: make(map[string]*Object),
	}
}

func (c *InMemoryClient) Query(ctx context.Context, query Query, options ...QueryOptions) (*ObjectQueryResult, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	var items []Object
	for _, obj := range c.store {
		if query.ResourceType != "" && !strings.EqualFold(obj.Metadata.ResourceType, query.ResourceType) {
			continue
		}
		if query.ScopeRecursive {
			if !strings.HasPrefix(strings.ToLower(obj.Metadata.RootScope), strings.ToLower(query.RootScope)) {
				continue
			}
		} else {
			if !strings.EqualFold(obj.Metadata.RootScope, query.RootScope) {
				continue
			}
		}
		items = append(items, *obj)
	}

	return &ObjectQueryResult{Items: items}, nil
}

func (c *InMemoryClient) Get(ctx context.Context, id string, options ...GetOptions) (*Object, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	obj, ok := c.store[strings.ToLower(id)]
	if !ok {
		return nil, &ErrNotFound{ID: id}
	}

	// Return a copy
	data, _ := json.Marshal(obj.Data)
	var dataCopy any
	_ = json.Unmarshal(data, &dataCopy)

	return &Object{
		Metadata: obj.Metadata,
		Data:     dataCopy,
		ETag:     obj.ETag,
	}, nil
}

func (c *InMemoryClient) Delete(ctx context.Context, id string, options ...DeleteOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := strings.ToLower(id)
	existing, ok := c.store[key]
	if !ok {
		return &ErrNotFound{ID: id}
	}

	if len(options) > 0 && options[0].ETag != "" {
		if existing.ETag != options[0].ETag {
			return &ErrConcurrency{ID: id}
		}
	}

	delete(c.store, key)
	return nil
}

func (c *InMemoryClient) Save(ctx context.Context, obj *Object, options ...SaveOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := strings.ToLower(obj.Metadata.ID)
	existing, exists := c.store[key]

	if len(options) > 0 && options[0].ETag != "" {
		if !exists {
			return &ErrNotFound{ID: obj.Metadata.ID}
		}
		if existing.ETag != options[0].ETag {
			return &ErrConcurrency{ID: obj.Metadata.ID}
		}
	}

	now := time.Now()
	if !exists {
		obj.Metadata.CreatedAt = now
	}
	obj.Metadata.UpdatedAt = now
	obj.ETag = uuid.New().String()

	// Store a copy
	data, _ := json.Marshal(obj.Data)
	var dataCopy any
	_ = json.Unmarshal(data, &dataCopy)

	c.store[key] = &Object{
		Metadata: obj.Metadata,
		Data:     dataCopy,
		ETag:     obj.ETag,
	}

	return nil
}
