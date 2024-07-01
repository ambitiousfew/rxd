package rxd

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

type Daemon interface {
	AddServices(services ...Service) error
	AddService(service Service) error
	Start(ctx context.Context) error
}

type daemon struct {
	services map[string]DaemonService
	started  atomic.Bool
	errC     chan error
}

// NewDaemon creates and return an instance of the reactive daemon
func NewDaemon(name string, options ...DaemonOption) Daemon {
	d := &daemon{
		services: make(map[string]DaemonService),
		errC:     make(chan error, 100),
		started:  atomic.Bool{},
	}

	for _, option := range options {
		option(d)
	}

	return d

}

func (d *daemon) Start(parent context.Context) error {
	if d.started.Swap(true) {
		return ErrDaemonStarted
	}

	if len(d.services) == 0 {
		return ErrNoServices
	}

	// daemon context
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	// signal watcher
	go func() {
		signalC := make(chan os.Signal, 1)
		signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(signalC)

		select {
		case <-ctx.Done():
			d.errC <- errors.New("daemon received context done signal")
		case <-signalC:
			d.errC <- errors.New("daemon os signal received")
		}
		// if we received a signal to stop, cancel the context
		cancel()
	}()

	go func() {
		for err := range d.errC {
			log.Println(err.Error())
		}
	}()

	var wg sync.WaitGroup
	// launch all services in their own routine.
	for _, ds := range d.services {
		wg.Add(1)

		go func(service DaemonService, handler ServiceHandler) {
			serviceCtx, serviceCancel := context.WithCancel(ctx)
			defer serviceCancel()

			handler.Handle(serviceCtx, service, d.errC)

			wg.Done()
		}(ds, ds.Handler)

	}

	// block, waiting for all services to exit their lifecycles.
	wg.Wait()
	// close the error channel, so the error handler routine can exit.
	close(d.errC)
	return nil
}

// AddServices adds a list of services to the daemon.
// if any service fails to be added, the error is logged and the next service is attempted.
// any services that fail likely are failing due to name overlap and will be skipped
// if daemon is already started, no new services can be added.
func (d *daemon) AddServices(services ...Service) error {
	for _, service := range services {
		err := d.addService(service)
		if err != nil {
			return err
		}
	}
	return nil
}

// AddService adds a service to the daemon.
// if the service fails to be added, the error will be returned.
func (d *daemon) AddService(service Service) error {
	return d.addService(service)
}

// addService is a helper function to add a service to the daemon.
func (d *daemon) addService(service Service) error {
	if d.started.Load() {
		return ErrAddingServiceOnceStarted
	}

	if service.Runner == nil {
		return ErrNilService
	}

	if service.Name == "" {
		return ErrNoServiceName
	}

	// add the service to the daemon services
	d.services[service.Name] = DaemonService{
		Name:    service.Name,
		Runner:  service.Runner,
		Handler: service.RunPolicy,
	}

	return nil
}
