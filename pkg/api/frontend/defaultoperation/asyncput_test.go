package defaultoperation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	"github.com/ryobi-project/ryobi/pkg/api/async"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/components/database"
	"github.com/ryobi-project/ryobi/pkg/components/queue"
)

type testResource struct {
	Name       string `json:"name"`
	Value      string `json:"value"`
}

func testConverter(body []byte) (*testResource, error) {
	var r testResource
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func testResponseConverter(r *testResource) (any, error) {
	return r, nil
}

func setupTestOptions() ctrl.Options {
	db := database.NewInMemoryClient()
	q := queue.NewInMemoryClient()
	sm := async.NewStatusManager(db, q, "localhost:9000")

	return ctrl.Options{
		Address:        "localhost:9000",
		DatabaseClient: db,
		StatusManager:  sm,
		ResourceType:   "test/resources",
	}
}

func TestDefaultAsyncPut_ReturnsAccepted(t *testing.T) {
	opts := setupTestOptions()
	resOpts := ctrl.ResourceOptions[testResource]{
		RequestConverter:  testConverter,
		ResponseConverter: testResponseConverter,
	}

	factory := NewDefaultAsyncPutFactory("/api/v1", resOpts)
	controller, err := factory(opts)
	if err != nil {
		t.Fatalf("factory failed: %v", err)
	}

	body := `{"name": "test-res", "value": "hello"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/test/resources/test-res", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "test-res")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	resp, err := controller.Run(req.Context(), w, req)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	// Apply the response to check status code
	err = resp.Apply(context.Background(), w, req)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202 Accepted, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location == "" {
		t.Error("expected Location header for async operation")
	}

	// Verify resource was saved to database
	_, getErr := opts.DatabaseClient.Get(context.Background(), "/api/v1/test/resources/test-res")
	if getErr != nil {
		t.Errorf("resource should be saved in database: %v", getErr)
	}
}

func TestDefaultAsyncPut_WithUpdateFilter(t *testing.T) {
	opts := setupTestOptions()

	filterCalled := false
	resOpts := ctrl.ResourceOptions[testResource]{
		RequestConverter:  testConverter,
		ResponseConverter: testResponseConverter,
		UpdateFilters: []ctrl.UpdateFilter[testResource]{
			func(ctx context.Context, newRes *testResource, oldRes *testResource, options *ctrl.Options) (v1.Response, error) {
				filterCalled = true
				if newRes.Value == "blocked" {
					return v1.NewBadRequestResponse("value 'blocked' is not allowed"), nil
				}
				return nil, nil
			},
		},
	}

	factory := NewDefaultAsyncPutFactory("/api/v1", resOpts)
	controller, _ := factory(opts)

	// Test blocked value
	body := `{"name": "test", "value": "blocked"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/test/resources/test", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	resp, err := controller.Run(req.Context(), w, req)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !filterCalled {
		t.Error("update filter should have been called")
	}

	// Apply and check it's a 400
	err = resp.Apply(context.Background(), w, req)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDefaultSyncPut_ReturnsOK(t *testing.T) {
	opts := setupTestOptions()
	resOpts := ctrl.ResourceOptions[testResource]{
		RequestConverter:  testConverter,
		ResponseConverter: testResponseConverter,
	}

	factory := NewDefaultSyncPutFactory("/api/v1", resOpts)
	controller, _ := factory(opts)

	body := `{"name": "test-env", "value": "dev"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/test/envs/test-env", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "test-env")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	resp, err := controller.Run(req.Context(), w, req)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	err = resp.Apply(context.Background(), w, req)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201 Created for new resource, got %d", w.Code)
	}
}
