package connections

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client communicates with the ryobid API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new API client.
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ListResponse is the standard list response format.
type ListResponse struct {
	Value []json.RawMessage `json:"value"`
}

// Get performs a GET request and returns the response body.
func (c *Client) Get(ctx context.Context, path string) ([]byte, int, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

// Put performs a PUT request with a JSON body.
func (c *Client) Put(ctx context.Context, path string, body any) ([]byte, int, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request body: %w", err)
	}
	return c.do(ctx, http.MethodPut, path, data)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) ([]byte, int, error) {
	return c.do(ctx, http.MethodDelete, path, nil)
}

// PollOperation polls an async operation URL until it completes.
func (c *Client) PollOperation(ctx context.Context, operationURL string, interval time.Duration) (*OperationStatus, error) {
	if interval == 0 {
		interval = 2 * time.Second
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		body, statusCode, err := c.do(ctx, http.MethodGet, operationURL, nil)
		if err != nil {
			return nil, err
		}

		if statusCode == http.StatusNotFound {
			time.Sleep(interval)
			continue
		}

		var status OperationStatus
		if err := json.Unmarshal(body, &status); err != nil {
			return nil, fmt.Errorf("failed to parse operation status: %w", err)
		}

		if status.IsTerminal() {
			return &status, nil
		}

		time.Sleep(interval)
	}
}

// OperationStatus represents an async operation status from the API.
type OperationStatus struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Error  *OperationError `json:"error,omitempty"`
}

// OperationError holds error details from an operation.
type OperationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// IsTerminal returns true if the operation is in a terminal state.
func (s *OperationStatus) IsTerminal() bool {
	switch s.Status {
	case "Succeeded", "Failed", "Canceled":
		return true
	}
	return false
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	url := path
	if len(path) > 0 && path[0] == '/' {
		url = c.baseURL + path
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}
