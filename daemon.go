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
	services        map[string]DaemonService
	started         atomic.Bool
	errC            chan error
	reportAliveSecs uint64
}

// NewDaemon creates and return an instance of the reactive daemon
func NewDaemon(name string, options ...DaemonOption) Daemon {
	d := &daemon{
		services:        make(map[string]DaemonService),
		errC:            make(chan error, 100),
		started:         atomic.Bool{},
		reportAliveSecs: 0,
	}

	for _, option := range options {
		option(d)
	}

	return d

}

func (d *daemon) Start(parent context.Context) error {
	// pre-start checks
	if d.started.Swap(true) {
		return ErrDaemonStarted
	}

	if len(d.services) == 0 {
		return ErrNoServices
	}

	// TODO: To eventually support running rxd on multiple platforms, we need to
	// abstract the notifier to be a part of the daemon configuration.
	// For now, we are only supporting systemd.
	// NOTE: Since service manager selection is part of the build runtime.
	// we will probably have to do this via mixture of global and init().

	// notifier := GetSystemNotifier(ctx) --- probably...
	notifier, err := NewSystemdNotifier(os.Getenv("NOTIFY_SOCKET"), d.reportAliveSecs)
	if err != nil {
		return err
	}

	// daemon child context from parent
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	errC := make(chan error, 1) // error channel for system notifier

	// Start the notifier, this will start the watchdog portion.
	// so we can notify systemd that we have not hung.
	err = notifier.Start(ctx, errC)
	if err != nil {
		return err
	}

	go func() {
		// error handler routine
		// closed after the wait group is done.
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-d.errC:
				// errors from the services
				log.Println(err.Error())
			case err := <-errC:
				// errors from the notifier
				log.Println(err.Error())
			}
		}
	}()

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

		// inform systemd that we are stopping/cleaning up
		// TODO: Test if this notify should happen before or after cancel()
		// since the watchdog notify continues to until the context is cancelled.
		err = notifier.Notify(NotifyStateStopping)
		if err != nil {
			d.errC <- err
		}
		// if we received a signal to stop, cancel the context
		cancel()
	}()

	var wg sync.WaitGroup
	// launch all services in their own routine.
	for _, ds := range d.services {
		wg.Add(1)

		// launch the service in its own routine
		go func(service DaemonService, handler ServiceHandler) {
			serviceCtx, serviceCancel := context.WithCancel(ctx)
			defer serviceCancel()

			// run the service according to the handler policy
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
