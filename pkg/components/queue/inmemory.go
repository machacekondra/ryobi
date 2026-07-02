package queue

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

const defaultVisibilityTimeout = 30 * time.Second

// InMemoryClient is an in-memory implementation of the queue Client for testing.
type InMemoryClient struct {
	mu       sync.Mutex
	messages []*Message
}

// NewInMemoryClient creates a new InMemoryClient.
func NewInMemoryClient() *InMemoryClient {
	return &InMemoryClient{}
}

func (c *InMemoryClient) Enqueue(ctx context.Context, msg *Message, options ...EnqueueOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	msg.EnqueueAt = time.Now()
	msg.NextVisibleAt = msg.EnqueueAt

	if len(options) > 0 && options[0].Delay > 0 {
		msg.NextVisibleAt = msg.EnqueueAt.Add(options[0].Delay)
	}

	data := make(json.RawMessage, len(msg.Data))
	copy(data, msg.Data)
	c.messages = append(c.messages, &Message{
		ID:            msg.ID,
		ContentType:   msg.ContentType,
		Data:          data,
		DequeueCount:  0,
		EnqueueAt:     msg.EnqueueAt,
		NextVisibleAt: msg.NextVisibleAt,
	})

	return nil
}

func (c *InMemoryClient) Dequeue(ctx context.Context, options ...DequeueOptions) (*Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	visibilityTimeout := defaultVisibilityTimeout
	if len(options) > 0 && options[0].VisibilityTimeout > 0 {
		visibilityTimeout = options[0].VisibilityTimeout
	}

	now := time.Now()
	for _, msg := range c.messages {
		if now.After(msg.NextVisibleAt) {
			msg.DequeueCount++
			msg.NextVisibleAt = now.Add(visibilityTimeout)
			return msg, nil
		}
	}

	return nil, nil
}

func (c *InMemoryClient) Complete(ctx context.Context, msg *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, m := range c.messages {
		if m.ID == msg.ID {
			c.messages = append(c.messages[:i], c.messages[i+1:]...)
			return nil
		}
	}

	return nil
}

func (c *InMemoryClient) Extend(ctx context.Context, msg *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, m := range c.messages {
		if m.ID == msg.ID {
			m.NextVisibleAt = time.Now().Add(defaultVisibilityTimeout)
			return nil
		}
	}

	return nil
}
