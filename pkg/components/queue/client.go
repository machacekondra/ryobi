package queue

import (
	"context"
	"encoding/json"
	"time"
)

// Client is the interface for a message queue.
type Client interface {
	Enqueue(ctx context.Context, msg *Message, options ...EnqueueOptions) error
	Dequeue(ctx context.Context, options ...DequeueOptions) (*Message, error)
	Complete(ctx context.Context, msg *Message) error
	Extend(ctx context.Context, msg *Message) error
}

// Message represents a queue message.
type Message struct {
	ID            string
	ContentType   string
	Data          json.RawMessage
	DequeueCount  int
	EnqueueAt     time.Time
	ExpireAt      time.Time
	NextVisibleAt time.Time
}

// EnqueueOptions are options for the Enqueue operation.
type EnqueueOptions struct {
	Delay time.Duration
}

// DequeueOptions are options for the Dequeue operation.
type DequeueOptions struct {
	VisibilityTimeout time.Duration
}
