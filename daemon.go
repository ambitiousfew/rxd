package rxd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"
)

type Daemon struct {
	// DaemonConfig is the daemon-level configuration.
	Config DaemonConfig
	// Services is a map of services managed by the daemon.
	services map[string]Service
	// serviceOpts is a map of service names to their respective service options.
	serviceOpts map[Servicer]*serviceOpts
	// statesC is a channel for all state updates from all services.
	statesC chan stateUpdate
	// log is the parent logger for the daemon.
	log *slog.Logger
	// started is a flag to track if the daemon has been started.
	started bool
}

func NewDaemon(config DaemonConfig) *Daemon {
	opts := &daemonOpts{
		log: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}

	// Apply all functional options to update defaults.
	for _, applyOption := range config.Opts {
		applyOption(opts)
	}

	// set a "daemon" attribute to the name of the daemon config name
	logger := opts.log.With("daemon", config.Name)

	daemon := &Daemon{
		Config:      config,
		services:    make(map[string]Service, 0),
		serviceOpts: make(map[Servicer]*serviceOpts),
		// serviceConfigs: make(map[string]ServiceConfig),
		// scRequests: make(chan serviceContextRequest, 1),

		log:     logger,
		started: false,
	}

	return daemon
}

func (d *Daemon) addService(s Service) error {
	// cancel setup call if it takes too long
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if ctx.Err() != nil {
		return ErrServiceSetupTimeout
	}

	if s.Conf.Name == "" {
		return ErrServiceNameRequired
	}

	_, ok := d.services[s.Conf.Name]
	if ok {
		return ErrServiceAlreadyExists
	}

	opts := &serviceOpts{}

	for _, applyOption := range s.Conf.RunOpts {
		applyOption(opts)
	}

	s.Conf.opts = opts
	d.services[s.Conf.Name] = s

	return nil
}

func (d *Daemon) AddService(s Service) error {
	return d.addService(s)
}

func (d *Daemon) AddServices(s ...Service) error {
	for _, service := range s {
		err := d.addService(service)
		if err != nil {
			return err
		}
	}
	return nil
}

// Start method refactored for the updated service lifecycle management.
func (d *Daemon) Start(ctx context.Context) error {
	if d.started {
		return ErrAlreadyStarted
	}

	d.started = true

	if len(d.services) < 1 {
		return ErrNoServices
	}

	// ensure states channel is created and buffered with enough space for all services and lifecycle transitions.
	d.statesC = make(chan stateUpdate, len(d.services)*4)

	statesDoneC := make(chan struct{})
	go func() {
		// states watcher routine, relays all state transitions.
		defer close(statesDoneC)
		d.log.Debug("states watcher has started")
		for state := range d.statesC {
			d.log.Debug("states watcher update", slog.String("service", state.service), slog.String("state", state.state.String()))
		}
		d.log.Debug("states watcher has exited")
	}()

	// allServicesCtx is the parent context for all services and is a child context of the callers's context.
	allServicesCtx, allServicesCancel := context.WithCancel(ctx)
	defer allServicesCancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, d.Config.Signals...)

	// signal watcher routine, cancels all services on signal received.
	go func() {
		select {
		case <-sigChan:
			// os signal received, cancel all services
			allServicesCancel()
		case <-allServicesCtx.Done():
			// all services have exited, stop this routine.
			return
		}
	}()

	var wg sync.WaitGroup
	for _, service := range d.services {
		wg.Add(1)
		// Start each service in a separate goroutine.
		go func(s Service) {
			defer wg.Done()
			serviceStateC := make(chan ServiceState, 1)          // channel for service state transitions from rxd (start, stop, restart, reload).
			serviceStateC <- Init                                // initial state for all services.
			d.runServiceBroker(allServicesCtx, s, serviceStateC) // blocks, handles service lifecycles.
		}(service)
	}

	// Block start until all services have exited.
	wg.Wait()
	close(d.statesC)
	<-statesDoneC // wait for states watcher to exit.
	return nil
}

// runServiceBroker acts as a broker for the service's lifecycle.
// it handles state requests coming from rxd and the callers service itself.
// this is so rxd can signal state transitions like start, stop, restart, reload
// and the caller can also trigger state transitions to Init, Idle, Run, Stop, Exit.
func (d *Daemon) runServiceBroker(parentCtx context.Context, s Service, stateC <-chan ServiceState) {
	serviceLog := d.log.With("service", s.Conf.Name)
	serviceLog.Debug("service broker has started")

	var hasStarted, hasStopped bool // flags to track if the service has started or stopped.

	states := map[ServiceState]func(context.Context) ServiceResponse{
		Init: s.Svc.Init,
		Idle: s.Svc.Idle,
		Run:  s.Svc.Run,
		Stop: s.Svc.Stop,
	}

	// caller asking to move to a new state by returning a new response from lifecycle method.
	returnedStateC := make(chan ServiceState, 1)

	// done channel for signalling when the service has exited its current state.
	doneC := make(chan struct{})

	var stopping bool // flag to track if the service is being shutdown.
	for !stopping {
		// Each new state is handled in a separate goroutine.
		// This allows for concurrent state handling and cancellation.
		serviceCtx, serviceCancel := context.WithCancel(parentCtx)

		var nextState ServiceState
		select {
		case <-parentCtx.Done(): // all services being cancelled/stopped.
			nextState = Exit
			stopping = true
		case nextState = <-stateC: // rxd requesting move to new state
		// TODO: add handling for reloading services.
		case nextState = <-returnedStateC: // caller requesting move to new state
		}

		// service has entered at least one lifecycle method.
		if hasStarted {
			// we received a new state to change to.
			serviceCancel()                                           // cancel the previous state context.
			<-doneC                                                   // give the routine a chance to complete before moving to the next state.
			serviceCtx, serviceCancel = context.WithCancel(parentCtx) // recreate service context and the cancel function.
			doneC = make(chan struct{})                               // re-create the service's done signal channel.
		}

		hasStarted = true

		// TODO: Might be able to limit using a semaphore here?

		// service-specific lifecycle routine
		go func() {
			// Each state transition is handled in a separate goroutine for concurrent handling and cancellation.
			defer close(doneC) // signal done.
			stateFunc, ok := states[nextState]
			if !ok {
				if nextState == Exit && !hasStopped {
					// if the next state is exit and we haven't already stopped, stop the service first.
					serviceLog.Debug("service entering 'stop' before exiting")
					stateFunc = states[Stop]
					hasStopped = true
				} else {
					// next state is an unknown state transition, log and return early.
					serviceLog.Debug("service error", slog.String("state", nextState.String()), slog.String("error", "state not found"))
					serviceCancel()
					return
				}
			}

			if nextState == Stop && hasStopped {
				// if the next state is stop and we have already stopped, return early.
				serviceLog.Debug("service response error", slog.String("state", nextState.String()), slog.String("error", "service has already stopped, exiting early"))
				serviceCancel()
				return
			}

			serviceLog.Debug("service entering state", slog.String("state", nextState.String()))
			// inform the states watcher what we are doing next.
			d.statesC <- stateUpdate{service: s.Conf.Name, state: nextState}

			// blocking run of lifecycle method happens here.
			result := stateFunc(serviceCtx) // Init, Idle, Run, Stop
			if result.Err != nil {
				serviceLog.Debug("service error", slog.String("state", nextState.String()), slog.String("error", result.Err.Error()))
			}

			// relay back to the service manager the next desired state.
			select {
			case <-serviceCtx.Done():
				return
			case returnedStateC <- result.Next:
			}
			serviceLog.Debug("service exiting state", slog.String("state", nextState.String()))
		}()

	}

	<-doneC // ensure all states have exited before returning.
	serviceLog.Debug("service broker has exited")
	// Service has exited.
}
