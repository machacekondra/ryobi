package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// OperationMethod represents an HTTP method for an operation.
type OperationMethod string

const (
	OperationGet    OperationMethod = "GET"
	OperationPut    OperationMethod = "PUT"
	OperationPatch  OperationMethod = "PATCH"
	OperationDelete OperationMethod = "DELETE"
	OperationList   OperationMethod = "LIST"
)

// HTTPMethod returns the HTTP method string.
func (m OperationMethod) HTTPMethod() string {
	switch m {
	case OperationList:
		return http.MethodGet
	default:
		return string(m)
	}
}

// OperationType identifies the type of operation being performed.
type OperationType struct {
	Type   string
	Method OperationMethod
}

// ProvisioningState represents the state of an async operation.
type ProvisioningState string

const (
	ProvisioningStateAccepted   ProvisioningState = "Accepted"
	ProvisioningStateUpdating   ProvisioningState = "Updating"
	ProvisioningStateDeleting   ProvisioningState = "Deleting"
	ProvisioningStateSucceeded  ProvisioningState = "Succeeded"
	ProvisioningStateFailed     ProvisioningState = "Failed"
	ProvisioningStateCanceled   ProvisioningState = "Canceled"
	ProvisioningStateProvisioning ProvisioningState = "Provisioning"
)

// IsTerminal returns true if the provisioning state is a terminal state.
func (s ProvisioningState) IsTerminal() bool {
	return s == ProvisioningStateSucceeded || s == ProvisioningStateFailed || s == ProvisioningStateCanceled
}

// DefaultRetryAfter is the default Retry-After interval for async operations.
const DefaultRetryAfter = 5 * time.Second

// Resource is the base type for all resources.
type Resource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// TrackedResource is a resource with metadata tracking.
type TrackedResource struct {
	Resource
	CreatedAt    time.Time `json:"createdAt,omitempty"`
	LastModified time.Time `json:"lastModified,omitempty"`
}

// ErrorResponse is the standard error response format.
type ErrorResponse struct {
	Error *ErrorDetails `json:"error"`
}

// ErrorDetails contains error information.
type ErrorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Target  string `json:"target,omitempty"`
}

// Standard error codes.
const (
	CodeInternal         = "Internal"
	CodeNotFound         = "NotFound"
	CodeConflict         = "Conflict"
	CodeBadRequest       = "BadRequest"
	CodeInvalidArgument  = "InvalidArgument"
)

// Response is the interface for API responses.
type Response interface {
	Apply(ctx context.Context, w http.ResponseWriter, req *http.Request) error
}

// JSONResponse is a simple JSON response.
type JSONResponse struct {
	StatusCode int
	Body       any
	Headers    map[string]string
}

// Apply writes the JSON response to the http.ResponseWriter.
func (r *JSONResponse) Apply(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	for k, v := range r.Headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.StatusCode)

	if r.Body != nil {
		return json.NewEncoder(w).Encode(r.Body)
	}
	return nil
}

// NewOKResponse creates a 200 OK response.
func NewOKResponse(body any) Response {
	return &JSONResponse{StatusCode: http.StatusOK, Body: body}
}

// NewCreatedResponse creates a 201 Created response.
func NewCreatedResponse(body any) Response {
	return &JSONResponse{StatusCode: http.StatusCreated, Body: body}
}

// NewAcceptedResponse creates a 202 Accepted response with operation location.
func NewAcceptedResponse(operationURL string) Response {
	return &JSONResponse{
		StatusCode: http.StatusAccepted,
		Headers: map[string]string{
			"Location":    operationURL,
			"Retry-After": "5",
		},
	}
}

// NewNoContentResponse creates a 204 No Content response.
func NewNoContentResponse() Response {
	return &JSONResponse{StatusCode: http.StatusNoContent}
}

// NewNotFoundResponse creates a 404 Not Found response.
func NewNotFoundResponse(id string) Response {
	return &JSONResponse{
		StatusCode: http.StatusNotFound,
		Body: ErrorResponse{
			Error: &ErrorDetails{
				Code:    CodeNotFound,
				Message: "resource not found: " + id,
			},
		},
	}
}

// NewBadRequestResponse creates a 400 Bad Request response.
func NewBadRequestResponse(message string) Response {
	return &JSONResponse{
		StatusCode: http.StatusBadRequest,
		Body: ErrorResponse{
			Error: &ErrorDetails{
				Code:    CodeBadRequest,
				Message: message,
			},
		},
	}
}

// NewInternalErrorResponse creates a 500 Internal Server Error response.
func NewInternalErrorResponse(err error) Response {
	return &JSONResponse{
		StatusCode: http.StatusInternalServerError,
		Body: ErrorResponse{
			Error: &ErrorDetails{
				Code:    CodeInternal,
				Message: err.Error(),
			},
		},
	}
}

// NewConflictResponse creates a 409 Conflict response.
func NewConflictResponse(message string) Response {
	return &JSONResponse{
		StatusCode: http.StatusConflict,
		Body: ErrorResponse{
			Error: &ErrorDetails{
				Code:    CodeConflict,
				Message: message,
			},
		},
	}
}

// RequestContext holds parsed information from the incoming request.
type RequestContext struct {
	ResourceID    string
	ResourceType  string
	OperationType OperationType
}

type contextKey string

const requestContextKey contextKey = "ryobi-request-context"

// WithRequestContext stores the RequestContext in the context.
func WithRequestContext(ctx context.Context, rc *RequestContext) context.Context {
	return context.WithValue(ctx, requestContextKey, rc)
}

// RequestContextFromContext retrieves the RequestContext from the context.
func RequestContextFromContext(ctx context.Context) *RequestContext {
	rc, _ := ctx.Value(requestContextKey).(*RequestContext)
	return rc
}
