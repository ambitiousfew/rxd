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
	// Services is a slice of all services managed by the daemon.
	Services []Service
	// serviceConfigs is a map of service names to their respective configurations.
	serviceConfigs map[string]ServiceConfig
	// serviceOpts is a map of service names to their respective service options.
	serviceOpts map[string]*serviceOpts
	// scRequests is a channel for managing service context requests.
	scRequests chan serviceContextRequest
	// log is the parent logger for the daemon.
	log *slog.Logger
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
	opts.log = opts.log.With("daemon", config.Name)

	daemon := &Daemon{
		Config:         config,
		Services:       make([]Service, 0),
		serviceConfigs: make(map[string]ServiceConfig),
		serviceOpts:    make(map[string]*serviceOpts),
		scRequests:     make(chan serviceContextRequest, 1),
		log:            opts.log,
	}

	return daemon
}

func (d *Daemon) addService(s Service) error {
	// cancel setup call if it takes too long
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conf := s.Setup(ctx)
	if _, ok := d.serviceConfigs[conf.Name]; ok {
		return ErrServiceAlreadyExists
	}

	opts := &serviceOpts{
		log: d.log,
	}

	for _, applyOption := range conf.RunOpts {
		applyOption(opts)
	}

	opts.log = opts.log.With("service", conf.Name)

	d.serviceConfigs[conf.Name] = conf
	d.serviceOpts[conf.Name] = opts
	d.Services = append(d.Services, s)
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
	if len(d.Services) < 1 {
		return ErrNoServices
	}

	// allServicesCtx is the parent context for all services and is a child context of the callers's context.
	allServicesCtx, allServicesCancel := context.WithCancel(ctx)
	defer allServicesCancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, d.Config.Signals...)

	// Start the service context request manager.
	go manageServiceContexts(d.scRequests)

	var wg sync.WaitGroup

	for _, service := range d.Services {
		wg.Add(1)
		// Start each service in a separate goroutine.
		go func(s Service) {
			defer wg.Done()
			runService(allServicesCtx, s)
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
func runService(parentCtx context.Context, s Service) {
	// service-specific cancellable context
	serviceCtx, serviceCancel := context.WithCancel(parentCtx)
	defer serviceCancel()

	conf := s.Setup(serviceCtx)
	log := conf.opts.log // service-specific logger

	// register this service's context/cancel with the service context manager
	// d.scRequests <- serviceContextRequest{
	// 	operation: AddContext,
	// 	service:   s,
	// 	context:   serviceCtx,
	// 	cancel:    serviceCancel,
	// }
	var result ServiceResponse
	state := Init // Always begin with the Init state.
	for state != Exit {
		switch state {
		case Init:
			log.Debug("service entering 'init'")
			result = s.Init(serviceCtx)
		case Idle:
			log.Debug("service entering 'idle'")
			result = s.Idle(serviceCtx)
		case Run:
			log.Debug("service entering 'run'")
			result = s.Run(serviceCtx)
		case Stop:
			log.Debug("service entering 'stop'")
			result = s.Stop(serviceCtx)
		}

		if result.Err != nil {
			log.Error("service error", slog.String("state", state.String()), slog.String("error", result.Err.Error()))
		}

		state = result.Next // Move to the next state as indicated by the service.
	}

	// service has exited, cancel its context.
	// if state == Exit {
	// // If moving to Exit, we are exiting all the way...
	// responseChan := make(chan *serviceContextResponse)
	// // send request to retrieve context cancel for service
	// d.scRequests <- serviceContextRequest{
	// 	operation: GetContext,
	// 	service:   s,
	// 	response:  responseChan,
	// }

	serviceCancel() // Cancel the service's context.
	// exit service wrapper.

	// response := <-responseChan
	// response.cancel() // Cancel the service's context.

	// }
}
