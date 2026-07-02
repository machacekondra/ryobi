package hosting

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

const ShutdownTimeout = time.Second * 10

// Service is an abstraction for a long-running subsystem.
type Service interface {
	Name() string
	Run(ctx context.Context) error
}

// Host manages the lifetimes and starting of Services.
type Host struct {
	Services    []Service
	LoggerValues []any
	TimeoutFunc func()
}

// LifecycleMessage is returned when a service terminates.
type LifecycleMessage struct {
	Name string
	Err  error
}

// RunAsync runs services asynchronously and returns channels for monitoring.
func (host *Host) RunAsync(ctx context.Context) (<-chan error, <-chan LifecycleMessage) {
	stopped := make(chan error, 1)
	serviceErrors := make(chan LifecycleMessage, len(host.Services))

	go func() {
		defer close(stopped)
		err := host.Run(ctx, serviceErrors)
		stopped <- err
	}()

	return stopped, serviceErrors
}

// Run starts all services, waits for them to finish, and returns an error if any fail.
func (host *Host) Run(ctx context.Context, serviceErrors chan<- LifecycleMessage) error {
	if serviceErrors != nil {
		defer close(serviceErrors)
	}

	if len(host.Services) == 0 {
		return errors.New("at least one service is required")
	}

	logger := logr.FromContextOrDiscard(ctx)
	logger = logger.WithValues(host.LoggerValues...)
	ctx = logr.NewContext(ctx, logger)

	messages := make(chan LifecycleMessage, len(host.Services))
	defer close(messages)

	running := map[string]bool{}
	for _, service := range host.Services {
		if _, ok := running[service.Name()]; ok {
			return fmt.Errorf("detected duplicate service %s", service.Name())
		}
		running[service.Name()] = true
	}

	for i := range host.Services {
		service := host.Services[i]
		logger.Info(fmt.Sprintf("Starting %s", service.Name()))

		go func() {
			defer func() {
				value := recover()
				if value != nil {
					err := fmt.Errorf("service %s panicked: %v", service.Name(), value)
					logger.Error(err, "recovered from panic")
					messages <- LifecycleMessage{Name: service.Name(), Err: err}
				}
			}()

			err := host.runService(ctx, service)
			messages <- LifecycleMessage{Name: service.Name(), Err: err}
		}()
	}

	timeout := make(chan struct{}, 1)
	go func() {
		<-ctx.Done()
		if host.TimeoutFunc != nil {
			host.TimeoutFunc()
		} else {
			time.Sleep(ShutdownTimeout)
		}
		timeout <- struct{}{}
		close(timeout)
	}()

	logger.Info("Started all services", "count", len(host.Services))

	for len(running) > 0 {
		select {
		case message := <-messages:
			delete(running, message.Name)
			if message.Err != nil {
				logger.Error(message.Err, fmt.Sprintf("Service %s terminated with error", message.Name))
				if serviceErrors != nil {
					serviceErrors <- message
				}
			} else {
				logger.Info(fmt.Sprintf("Service %s terminated gracefully", message.Name))
			}
		case <-timeout:
			names := []string{}
			for k := range running {
				names = append(names, k)
			}
			sort.Strings(names)
			err := fmt.Errorf("shutdown timeout reached while the following services are still running: %s", strings.Join(names, ", "))
			logger.Error(err, "Shutdown timeout reached")
			return err
		}
	}

	return nil
}

func (host *Host) runService(ctx context.Context, service Service) error {
	logger := logr.FromContextOrDiscard(ctx)
	logger = logger.WithName(service.Name())
	ctx = logr.NewContext(ctx, logger)

	err := service.Run(ctx)
	if err == ctx.Err() {
		return nil
	}
	return err
}
