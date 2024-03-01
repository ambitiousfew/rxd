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
	// scRequests is a channel for managing service context requests.
	scRequests chan serviceContextRequest
	// log is the parent logger for the daemon.
	log *slog.Logger

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
		scRequests: make(chan serviceContextRequest, 1),
		log:        logger,
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

	// allServicesCtx is the parent context for all services and is a child context of the callers's context.
	allServicesCtx, allServicesCancel := context.WithCancel(ctx)
	defer allServicesCancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, d.Config.Signals...)

	var wg sync.WaitGroup

	for name, service := range d.services {
		wg.Add(1)
		// Start each service in a separate goroutine.
		go func(s Service) {
			defer wg.Done()
			d.log.Debug("starting service", slog.String("name", name))
			d.runService(allServicesCtx, s)
		}(service)
	}

	// signal watcher routine, cancels all services on signal received.
	go func() {
		select {
		case <-sigChan:
			// os signal received, cancel all services
			allServicesCancel()
		case <-ctx.Done():
			// user cancelled context, cancel all services
			return
		case <-allServicesCtx.Done():
			// all services have exited, stop this routine.
			return
		}
	}()

	// Block start until all services have exited.
	wg.Wait()

	// cleanup service context request manager
	close(d.scRequests)

	return nil
}

// runService is a wrapper for running a service in a separate goroutine.
// It manages the service lifecycle and context.
func (d *Daemon) runService(parentCtx context.Context, s Service) {
	// service-specific cancellable context
	serviceCtx, serviceCancel := context.WithCancel(parentCtx)
	defer serviceCancel()

	serviceLog := d.log.With("service", s.Conf.Name)

	// Intracom TODO: Report service state as entering 'setup'...

	state := Init // Always begin with the Init state.

	var hasStopped bool // track if stop has been called before.

	var result ServiceResponse
	for state != Exit {
		// Intracom TODO: Report the next service state we will enter...

		switch state {
		case Init:
			serviceLog.Debug("service entering 'init'")
			result = s.Svc.Init(serviceCtx)

		case Idle:
			serviceLog.Debug("service entering 'idle'")
			result = s.Svc.Idle(serviceCtx)
		case Run:
			serviceLog.Debug("service entering 'run'")
			result = s.Svc.Run(serviceCtx)
			if s.Conf.opts.runPolicy == RunOnce {
				// if the service is set to run once, we will move to the exit state after running.
				result.Next = Exit
			}
		case Stop:
			serviceLog.Debug("service entering 'stop'")
			result = s.Svc.Stop(serviceCtx)
			hasStopped = true
		}

		if result.Err != nil {
			serviceLog.Error("service error", slog.String("state", state.String()), slog.String("error", result.Err.Error()))
		}

		if hasStopped && result.Next == Stop {
			// if the service has been stopped and the next state is also stop, we will not continue.
			serviceLog.Error("service response error", slog.String("state", state.String()), slog.String("error", "service has already stopped"))
			break
		}

		if hasStopped && result.Next != Exit {
			// if the service has been stopped but we are not planning to exit, reset the stopped flag.
			hasStopped = false
		}

		state = result.Next // Move to the next state as indicated by the service.
	}

	if !hasStopped {
		// ensure we perform a final stop if we haven't already.
		result = s.Svc.Stop(serviceCtx)
		if result.Err != nil {
			serviceLog.Error("service error", slog.String("state", state.String()), slog.String("error", result.Err.Error()))
		}
	}

	// Cancel the service's context.
	serviceCancel()
	serviceLog.Debug("service has exited")
	// Service has exited.
}
