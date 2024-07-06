package rxd

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/ambitiousfew/rxd/intracom"
	"github.com/ambitiousfew/rxd/log"
)

type Daemon interface {
	AddServices(services ...Service) error
	AddService(service Service) error
	Start(ctx context.Context) error
}

type daemon struct {
	name            string                           // name of the daemon will be used in logging
	signals         []os.Signal                      // OS signals you want your daemon to listen for
	services        map[string]daemonService         // map of service name to struct carrying the service runner and name.
	handlers        map[string]ServiceHandler        // map of service name to service handler that will run the service runner methods.
	started         atomic.Bool                      // flag to indicate if the daemon has been started
	reportAliveSecs uint64                           // system service manager alive report timeout in seconds aka watchdog timeout
	logC            chan DaemonLog                   // log channel for the daemon to log service logs to
	logger          log.Logger                       // logger for the daemon
	icStates        intracom.Intracom[ServiceStates] // intracom comms bus for states
}

// NewDaemon creates and return an instance of the reactive daemon
func NewDaemon(name string, log log.Logger, options ...DaemonOption) Daemon {
	d := &daemon{
		name:            name,
		signals:         []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		services:        make(map[string]daemonService),
		handlers:        make(map[string]ServiceHandler),
		logC:            make(chan DaemonLog, 100),
		started:         atomic.Bool{},
		reportAliveSecs: 0,
		logger:          log,
		icStates:        intracom.New[ServiceStates]("rxd-states", log),
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

	// daemon child context from parent
	dctx, dcancel := context.WithCancel(parent)
	defer dcancel()

	// TODO: To eventually support running rxd on multiple platforms, we need to
	// abstract the notifier to be a part of the daemon configuration.
	// notifier := GetSystemNotifier(ctx) --- probably...
	// For now, we are only supporting linux - systemd.
	// NOTE: Since service manager selection is part of the build runtime.
	// we will probably have to do this via mixture of global and init().
	notifier, err := NewSystemdNotifier(os.Getenv("NOTIFY_SOCKET"), d.reportAliveSecs)
	if err != nil {
		return err
	}

	// Start the notifier, this will start the watchdog portion.
	// so we can notify systemd that we have not hung.
	err = notifier.Start(dctx, d.logC)
	if err != nil {
		return err
	}

	// --- Daemon Service(s) Log Handler ---
	// listens for logs from services via channel and logs them to the daemon logger.
	logDoneC := make(chan struct{})
	go func() {
		// error handler routine
		// closed after the wait group is done.
		for entry := range d.logC {
			d.logger.Log(entry.Level, entry.Message, log.String("service", entry.Name))
		}
		close(logDoneC)
	}()

	// --- Daemon Signal Watcher ---
	// listens for signals to stop the daemon such as OS signals or context done.
	go func() {
		signalC := make(chan os.Signal, 1)
		signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(signalC)

		select {
		case <-dctx.Done():
			d.logger.Log(log.LevelNotice, "daemon context done", log.String("rxd", "signal-watcher"))
		case <-signalC:
			d.logger.Log(log.LevelNotice, "daemon received signal to stop", log.String("rxd", "signal-watcher"))
		}

		// inform systemd that we are stopping/cleaning up
		// TODO: Test if this notify should happen before or after cancel()
		// since the watchdog notify continues to until the context is cancelled.
		err = notifier.Notify(NotifyStateStopping)
		if err != nil {
			d.logger.Log(log.LevelError, "error sending 'stopping' notification", log.String("rxd", "systemd-notifier"))
		}
		// if we received a signal to stop, cancel the context
		dcancel()
	}()

	// --- Service States Watcher ---
	statesDoneC := make(chan struct{})
	stateUpdateC := make(chan StateUpdate, len(d.services)*4)
	go func() {
		statesC, unregister := d.icStates.Register(parent, internalServiceStates)

		states := make(ServiceStates, len(d.services))
		for name := range d.services {
			states[name] = StateExit
		}

		// states watcher routine should be closed after all services have exited.
		for update := range stateUpdateC {
			if current, ok := states[update.Name]; ok && current != update.State {
				d.logger.Log(log.LevelDebug, "service state update", log.String("service", update.Name), log.String("state", update.State.String()))
			}
			// update the state of the service only if it changed.
			states[update.Name] = update.State

			// send the updated states to the intracom bus
			statesC <- states.copy()
		}
		unregister()
		close(statesDoneC)
	}()

	var dwg sync.WaitGroup
	// launch all services in their own routine.
	for _, service := range d.services {
		dwg.Add(1)

		handler := d.handlers[service.name]
		if err != nil && handler == nil {
			// TODO: Should we be doing pre-flight checks?
			// is it better to log the error and still try to start the daemon with the services that dont error
			// or is it better to fail fast and exit the daemon with an error?
			d.logger.Log(log.LevelError, "error getting handler for service", log.String("service", service.name))
			continue
		}

		// each service is handled in its own routine.
		go func(wg *sync.WaitGroup, ctx context.Context, svc daemonService, h ServiceHandler, stateC chan<- StateUpdate) {
			sctx, scancel := newServiceContextWithCancel(ctx, svc.name, d.logC, d.icStates)
			// run the service according to the handler policy
			h.Handle(sctx, svc.runner, stateC)
			scancel()
			wg.Done()

		}(&dwg, dctx, service, handler, stateUpdateC)
	}

	err = notifier.Notify(NotifyStateReady)
	if err != nil {
		d.logger.Log(log.LevelError, "error sending 'ready' notification", log.String("rxd", "systemd-notifier"))
	}
	// block, waiting for all services to exit their lifecycles.
	dwg.Wait()

	d.logger.Log(log.LevelDebug, "cleaning up log and states channels", log.String("rxd", d.name))
	// close the error channel to signal the error handler routine to exit
	// close(d.errC)
	close(d.logC)
	close(stateUpdateC)
	// wait for logging routine to empty the log channel and writing to logger
	<-logDoneC
	<-statesDoneC

	err = d.icStates.Close()
	if err != nil {
		d.logger.Log(log.LevelError, "error closing states intracom", log.String("rxd", "intracom"))
	}
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

	if service.Handler == nil {
		service.Handler = DefaultHandler{}
	}

	// add the service to the daemon services
	d.services[service.Name] = daemonService{
		name:   service.Name,
		runner: service.Runner,
	}

	// add the handler to a similar map of service name to handlers
	d.handlers[service.Name] = service.Handler

	return nil
}
