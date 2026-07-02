package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"

	v1 "github.com/ryobi-project/ryobi/pkg/api/v1"
	"github.com/ryobi-project/ryobi/pkg/api/async"
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/components/database"
	"github.com/ryobi-project/ryobi/pkg/components/hosting"
	"github.com/ryobi-project/ryobi/pkg/components/queue"
	"github.com/ryobi-project/ryobi/pkg/gateway"
	"github.com/ryobi-project/ryobi/pkg/recipes/terraform"
	"github.com/ryobi-project/ryobi/pkg/recipes/terraform/config/backends"
	backendctrl "github.com/ryobi-project/ryobi/pkg/resources/backend/controller"
	"github.com/ryobi-project/ryobi/pkg/resources/datamodel"
	"github.com/ryobi-project/ryobi/pkg/resources/setup"
	"github.com/ryobi-project/ryobi/pkg/version"
)

func main() {
	logger := stdr.New(nil)
	logger.Info("Starting ryobid", "version", version.Version, "commit", version.Commit)

	address := "0.0.0.0:9000"
	if addr := os.Getenv("RYOBI_ADDRESS"); addr != "" {
		address = addr
	}

	// Initialize components
	db := database.NewInMemoryClient()
	q := queue.NewInMemoryClient()
	sm := async.NewStatusManager(db, q, address)

	ctrlOpts := ctrl.Options{
		Address:        address,
		DatabaseClient: db,
		StatusManager:  sm,
		ResourceType:   "root",
	}

	// Set up router
	router := gateway.NewRouter(ctrlOpts, logger)
	setup.SetupRoutes(router, ctrlOpts)

	// Set up recipe engine
	terraformPath := os.Getenv("TERRAFORM_PATH")
	if terraformPath == "" {
		terraformPath = "terraform"
	}
	tfRootDir := os.Getenv("RYOBI_TF_ROOT_DIR")
	if tfRootDir == "" {
		tfRootDir = "/var/lib/ryobi/terraform"
	}
	var stateBackend backends.Backend
	if dbURL := os.Getenv("RYOBI_DB_URL"); dbURL != "" {
		stateBackend = backends.NewPostgresBackend(dbURL)
	} else {
		stateBackend = backends.NewLocalBackend(tfRootDir)
	}
	tfExecutor := terraform.NewExecutor(terraformPath, stateBackend)
	recipeEngine := terraform.NewRecipeEngine(tfExecutor, tfRootDir)

	// Set up async worker with resource handlers
	registry := async.NewControllerRegistry()
	registry.Register(datamodel.ResourceResourceType, v1.OperationPut, backendctrl.NewDeployResource(db, recipeEngine))
	registry.Register(datamodel.ResourceResourceType, v1.OperationDelete, backendctrl.NewDeleteResource(db, recipeEngine))
	worker := async.NewWorker(q, registry, sm)

	// Create HTTP server service
	apiServer := &httpService{
		name:    "api-server",
		address: address,
		handler: router.Handler(),
		logger:  logger,
	}

	// Start host
	host := &hosting.Host{
		Services: []hosting.Service{apiServer, worker},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx = logr.NewContext(ctx, logger)

	logger.Info(fmt.Sprintf("Listening on %s", address))
	_, _ = host.RunAsync(ctx)
	<-ctx.Done()
	logger.Info("Shutting down")
}

type httpService struct {
	name    string
	address string
	handler http.Handler
	logger  logr.Logger
}

func (s *httpService) Name() string {
	return s.name
}

func (s *httpService) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:    s.address,
		Handler: s.handler,
	}

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
