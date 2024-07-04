package rxd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

type Daemon interface {
	AddServices(services ...DaemonService) error
	AddService(service DaemonService) error
	Start(ctx context.Context) error
	Logger() Logger
}

type daemon struct {
	services        map[string]service
	handlers        map[string]ServiceHandler
	started         atomic.Bool
	errC            chan error
	reportAliveSecs uint64
	log             Logger
}

// NewDaemon creates and return an instance of the reactive daemon
func NewDaemon(name string, options ...DaemonOption) Daemon {
	// default logger
	dl := NewDefaultLogger(LogLevelInfo)

	d := &daemon{
		services:        make(map[string]service),
		handlers:        make(map[string]ServiceHandler),
		errC:            make(chan error, 100),
		started:         atomic.Bool{},
		reportAliveSecs: 0,
		log:             dl,
	}

	for _, option := range options {
		option(d)
	}

	return d

}

func (d *daemon) Logger() Logger {
	return d.log
}

func (d *daemon) Start(parent context.Context) error {
	// pre-start checks
	if d.started.Swap(true) {
		return ErrDaemonStarted
	}

	if len(d.services) == 0 {
		return ErrNoServices
	}

	// daemon child context from parent
	daemonCtx, daemonCancel := context.WithCancel(parent)
	defer daemonCancel()

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

	// Start the notifier, this will start the watchdog portion.
	// so we can notify systemd that we have not hung.
	err = notifier.Start(daemonCtx, d.errC)
	if err != nil {
		return err
	}

	errDoneC := make(chan struct{})
	go func() {
		// error handler routine
		// closed after the wait group is done.
		for err := range d.errC {
			// errors from the services
			d.log.Error("daemon error", map[string]any{"error": err})
		}
		close(errDoneC)
	}()

	// signal watcher
	go func() {
		signalC := make(chan os.Signal, 1)
		signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(signalC)

		select {
		case <-daemonCtx.Done():
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
		daemonCancel()
	}()

	var dwg sync.WaitGroup
	// launch all services in their own routine.
	for _, svc := range d.services {
		dwg.Add(1)

		handler := d.handlers[svc.Name]
		if err != nil && handler == nil {
			// TODO: daemon log error or something? or maybe preflight checks before this?
			continue
		}

		// launch the service in its own routine
		go func(wg *sync.WaitGroup, ctx context.Context, s service, h ServiceHandler) {
			sctx, scancel := NewServiceContextWithCancel(ctx, s.Name, d.log)
			defer scancel()

			// run the service according to the handler policy
			h.Handle(sctx, s, d.errC)
			wg.Done()

		}(&dwg, daemonCtx, svc, handler)

	}

	err = notifier.Notify(NotifyStateReady)
	if err != nil {
		d.errC <- err
	}
	// block, waiting for all services to exit their lifecycles.
	dwg.Wait()
	// close the error channel to signal the error handler routine to exit
	close(d.errC)
	// wait for error handler to finish emptying the error channel and writing to log
	<-errDoneC
	return nil
}

// AddServices adds a list of services to the daemon.
// if any service fails to be added, the error is logged and the next service is attempted.
// any services that fail likely are failing due to name overlap and will be skipped
// if daemon is already started, no new services can be added.
func (d *daemon) AddServices(services ...DaemonService) error {
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
func (d *daemon) AddService(service DaemonService) error {
	return d.addService(service)
}

// addService is a helper function to add a service to the daemon.
func (d *daemon) addService(svc DaemonService) error {
	if d.started.Load() {
		return ErrAddingServiceOnceStarted
	}

	if svc.Runner == nil {
		return ErrNilService
	}

	if svc.Name == "" {
		return ErrNoServiceName
	}

	if svc.Handler == nil {
		svc.Handler = DefaultHandler{}
	}

	// add the service to the daemon services
	d.services[svc.Name] = service{
		Name:   svc.Name,
		Runner: svc.Runner,
	}

	// add the handler to a similar map of service name to handlers
	d.handlers[svc.Name] = svc.Handler

	return nil
}
