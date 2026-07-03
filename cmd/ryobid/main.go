package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"google.golang.org/grpc"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	"github.com/ryobi-project/ryobi/pkg/api/async"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/components/database"
	"github.com/ryobi-project/ryobi/pkg/components/hosting"
	"github.com/ryobi-project/ryobi/pkg/components/queue"
	"github.com/ryobi-project/ryobi/pkg/gateway"
	"github.com/ryobi-project/ryobi/pkg/grpcapi"
	"github.com/ryobi-project/ryobi/pkg/placement"
	adminUI "github.com/ryobi-project/ryobi/pkg/ui/admin"
	portalUI "github.com/ryobi-project/ryobi/pkg/ui/portal"
	"github.com/ryobi-project/ryobi/pkg/resources/datamodel"
	"github.com/ryobi-project/ryobi/pkg/resources/setup"
	"github.com/ryobi-project/ryobi/pkg/version"
)

func main() {
	logger := stdr.New(nil)
	logger.Info("Starting ryobid", "version", version.Version, "commit", version.Commit)

	httpAddress := "0.0.0.0:9000"
	if addr := os.Getenv("RYOBI_ADDRESS"); addr != "" {
		httpAddress = addr
	}
	grpcAddress := "0.0.0.0:9001"
	if addr := os.Getenv("RYOBI_GRPC_ADDRESS"); addr != "" {
		grpcAddress = addr
	}

	// Initialize components
	db := database.NewInMemoryClient()
	q := queue.NewInMemoryClient()
	sm := async.NewStatusManager(db, q, httpAddress)

	ctrlOpts := ctrl.Options{
		Address:        httpAddress,
		DatabaseClient: db,
		StatusManager:  sm,
		ResourceType:   "root",
	}

	// Set up gRPC environment server
	envServer := grpcapi.NewEnvironmentServer(db, sm, logger)

	// Seed built-in resource types
	seedResourceTypes(db, logger)

	// Set up HTTP router
	router := gateway.NewRouter(ctrlOpts, logger)
	setup.SetupRoutes(router, ctrlOpts)

	// Mount UI SPAs
	router.MountSPA("/portal", portalUI.Assets)
	router.MountSPA("/admin", adminUI.Assets)

	// Set up placement engine and async worker
	placementEngine := placement.NewEngine()
	dispatcher := grpcapi.NewResourceDispatcher(db, envServer, placementEngine)
	registry := async.NewControllerRegistry()
	registry.Register(datamodel.ResourceResourceType, v1.OperationPut, dispatcher)
	registry.Register(datamodel.ResourceResourceType, v1.OperationDelete, dispatcher)
	worker := async.NewWorker(q, registry, sm)

	// Services
	apiServer := &httpService{
		name:    "api-server",
		address: httpAddress,
		handler: router.Handler(),
		logger:  logger,
	}
	grpcService := &grpcServerService{
		name:      "grpc-server",
		address:   grpcAddress,
		envServer: envServer,
		logger:    logger,
	}

	host := &hosting.Host{
		Services: []hosting.Service{apiServer, grpcService, worker},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx = logr.NewContext(ctx, logger)

	logger.Info(fmt.Sprintf("HTTP API listening on %s", httpAddress))
	logger.Info(fmt.Sprintf("gRPC listening on %s", grpcAddress))
	_, _ = host.RunAsync(ctx)
	<-ctx.Done()
	logger.Info("Shutting down")
}

// httpService wraps an HTTP server as a hosting.Service.
type httpService struct {
	name    string
	address string
	handler http.Handler
	logger  logr.Logger
}

func (s *httpService) Name() string { return s.name }

func (s *httpService) Run(ctx context.Context) error {
	server := &http.Server{Addr: s.address, Handler: s.handler}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// grpcServerService wraps a gRPC server as a hosting.Service.
type grpcServerService struct {
	name      string
	address   string
	envServer *grpcapi.EnvironmentServer
	logger    logr.Logger
}

func (s *grpcServerService) Name() string { return s.name }

func (s *grpcServerService) Run(ctx context.Context) error {
	lis, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.address, err)
	}

	grpcServer := grpc.NewServer()
	grpcapi.RegisterEnvironmentServiceServer(grpcServer, s.envServer)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	s.logger.Info("gRPC server started", "address", s.address)
	return grpcServer.Serve(lis)
}

func seedResourceTypes(db database.Client, logger logr.Logger) {
	types := []struct {
		name        string
		description string
		icon        string
		category    string
	}{
		{"Ryobi.Compute/containers", "Kubernetes Deployment managed via Terraform", "🐳", "compute"},
		{"Ryobi.Compute/virtualMachines", "KubeVirt Virtual Machine managed via Terraform", "🖥️", "compute"},
	}

	ctx := context.Background()
	for _, rt := range types {
		id := "/api/v1/ryobi/resource-types/" + rt.name
		if _, err := db.Get(ctx, id); err == nil {
			continue // already exists
		}
		obj := &database.Object{
			Metadata: database.Metadata{
				ID:           id,
				ResourceType: datamodel.ResourceTypeDefResourceType,
				RootScope:    "/api/v1",
			},
			Data: map[string]any{
				"name": rt.name,
				"type": datamodel.ResourceTypeDefResourceType,
				"properties": map[string]any{
					"description": rt.description,
					"icon":        rt.icon,
					"category":    rt.category,
				},
			},
		}
		if err := db.Save(ctx, obj); err != nil {
			logger.Error(err, "Failed to seed resource type", "name", rt.name)
		} else {
			logger.Info("Seeded resource type", "name", rt.name)
		}
	}

	// Seed catalog items
	catalogItems := []struct {
		name string
		data map[string]any
	}{
		{"nginx-webserver", map[string]any{
			"name": "nginx-webserver",
			"type": datamodel.CatalogItemResourceType,
			"properties": map[string]any{
				"description": "Nginx web server with configurable replicas",
				"icon":        "🌐",
				"category":    "web",
				"resources": []any{
					map[string]any{
						"name": "nginx",
						"type": "Ryobi.Compute/containers",
						"parameters": map[string]any{
							"name":           "nginx",
							"image":          "nginx:1.25-alpine",
							"replicas":       2,
							"ports":          []any{map[string]any{"container_port": 80}},
							"cpu_request":    "100m",
							"memory_request": "64Mi",
						},
					},
				},
				"parameters": []any{
					map[string]any{"name": "replicas", "description": "Number of nginx replicas", "type": "number", "default": 2},
					map[string]any{"name": "image", "description": "Container image", "type": "string", "default": "nginx:1.25-alpine"},
				},
			},
		}},
	}

	for _, ci := range catalogItems {
		id := "/api/v1/ryobi/catalog-items/" + ci.name
		if _, err := db.Get(ctx, id); err == nil {
			continue
		}
		obj := &database.Object{
			Metadata: database.Metadata{
				ID:           id,
				ResourceType: datamodel.CatalogItemResourceType,
				RootScope:    "/api/v1",
			},
			Data: ci.data,
		}
		if err := db.Save(ctx, obj); err != nil {
			logger.Error(err, "Failed to seed catalog item", "name", ci.name)
		} else {
			logger.Info("Seeded catalog item", "name", ci.name)
		}
	}
}
