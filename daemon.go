package rxd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/ambitiousfew/intracom"
	"golang.org/x/exp/slog"
)

type daemon struct {
	services sync.Map // map[string]*ServiceContext
	total    int      // total number of services added to daemon

	conf DaemonConfig

	iSignals *intracom.Intracom[rxdSignal] // signals from daemon to manager
	iStates  *intracom.Intracom[States]    // services state updates to manager

	log *slog.Logger

	started atomic.Bool
}

// NewDaemon creates and return an instance of the reactive daemon
func NewDaemon(conf DaemonConfig) *daemon {
	logger := slog.Default()

	if conf.LogHandler != nil {
		logger = slog.New(conf.LogHandler)
	}

	logger = logger.With("rxd", conf.Name)

	iSignals := intracom.New[rxdSignal]("rxd-signals")
	iStates := intracom.New[States]("rxd-states")

	if conf.IntracomLogHandler != nil {
		iSignals.SetLogHandler(conf.IntracomLogHandler)
		iStates.SetLogHandler(conf.IntracomLogHandler)
	}

	return &daemon{
		services: sync.Map{},
		total:    0,
		conf:     conf,

		iSignals: iSignals,
		iStates:  iStates,

		log: logger,

		started: atomic.Bool{},
	}
}

// AddService adds a service to the daemon.
// if the service fails to be added, the error will be returned.
func (d *daemon) AddService(s *ServiceContext) error {
	return d.addService(s)

}

// AddServices adds a list of services to the daemon.
// if any service fails to be added, the service will be skipped and the error will be logged.
func (d *daemon) AddServices(services ...*ServiceContext) error {
	for _, service := range services {
		err := d.addService(service)
		if err != nil {
			d.log.Error(err.Error())
			continue
		}
	}
	return nil
}

func (d *daemon) Start(ctx context.Context) error {
	if d.started.Load() {
		return fmt.Errorf("daemon is already running")
	}

	if d.total < 1 {
		return fmt.Errorf("no services were added prior to starting the daemon")
	}

	d.started.Store(true)

	// start an intracom instance for daemon to communicate with manager
	err := d.iSignals.Start()
	defer d.iStates.Close() // close the internal states intracom
	if err != nil {
		return err
	}

	// start an intracom instance for services to communicate with manager
	err = d.iStates.Start()
	if err != nil {
		return err
	}
	defer d.iSignals.Close() // close the internal signals intracom

	// register the internal states topic for use by the service state watcher to push states to services.
	statePublishC, unregisterStateC := d.iStates.Register(internalServiceStates)
	defer unregisterStateC()

	publishSignalC, unregisterSignalC := d.iSignals.Register(internalSignalsManager)
	defer unregisterSignalC()
	var wg sync.WaitGroup

	wg.Add(2) // signal watcher and manager

	go d.signalWatcher(&wg, ctx, publishSignalC) // OS Signal watcher routine.
	go d.manager(&wg, statePublishC)             // handles starting services and watching for signal to shutdown

	wg.Wait() // wait for signal watcher and manager to finish

	d.log.Debug("manager and signal watcher have stopped running, daemon is exiting")

	d.started.Store(false)
	return nil
}

// addService is a helper function to add a service to the daemon.
func (d *daemon) addService(s *ServiceContext) error {
	if d.started.Load() {
		return fmt.Errorf("cannot add a service once the daemon is started")
	}

	// ensure the service name hasn't already been added.
	if service, exists := d.services.Load(s.Name); exists {
		return fmt.Errorf("service with the name '%s' already exists", service.(*ServiceContext).Name)
	}

	d.services.Store(s.Name, s)
	d.total++

	return nil
}

func (d *daemon) manager(wg *sync.WaitGroup, statePublishC chan<- States) {
	defer wg.Done()
	// create a channel for each service to publish state updates to state watcher
	stateUpdateC := make(chan StateUpdate, d.total*2)
	defer close(stateUpdateC)

	doneC := make(chan struct{})
	go func() {
		defer close(doneC)
		// watch for internal signals from daemon to manager
		signalC, unsubscribe := d.iSignals.Subscribe(&intracom.SubscriberConfig{
			Topic:         internalSignalsManager,
			ConsumerGroup: "manager",
			BufferSize:    1,
			BufferPolicy:  intracom.DropNone,
		})
		defer unsubscribe()

		<-signalC // if we receive a signal from daemon signal watcher, we are done running.
		d.log.Debug("manager received stop signal from daemon")

		d.services.Range(func(anyName, anyService interface{}) bool {
			service, ok := anyService.(*ServiceContext)
			if !ok {
				d.log.Error("failed to stop service", "name", anyName, "error", "failed to cast service to *ServiceContext")
				return false
			}
			service.shutdown()
			return true
		})

		d.log.Debug("manager signaled shutdown to all services", "total", d.total)
	}()

	// a channel to signal the service state watcher to stop.
	stateWatcherStopC := make(chan struct{})

	// launch the service state watcher routine, signals on doneC when routine has exited.
	watcherDoneC := d.serviceStateWatcher(statePublishC, stateUpdateC, stateWatcherStopC)

	// services wait group
	var swg sync.WaitGroup

	var totalStarted int

	d.services.Range(func(name, anyService interface{}) bool {
		service, ok := anyService.(*ServiceContext) // all services must be of type *ServiceContext
		if !ok {
			d.log.Error("failed to start service", "name", name, "error", "failed to cast service to *ServiceContext")
			return false
		}
		// service := service                                // rebind loop variable
		service.iStates = d.iStates                       // attach the manager's internal states to each service
		service.Log = d.log.With("service", service.Name) // attach child logger instance with service name

		swg.Add(1)
		// Start each service in its own routine logic / conditional lifecycle.
		go startService(&swg, stateUpdateC, service)
		totalStarted++
		return true
	})

	d.log.Debug("manager started services", "total", totalStarted)
	swg.Wait() // blocks until all services have exited their lifecycles

	// all services have exited their lifecycles at this point.
	close(stateWatcherStopC) // signal to state watcher to stop
	<-watcherDoneC           // wait for state watcher to signal done

}

// serviceStateWatcher is run in its own routine by daemon to listen for updates from services as they change lifecycles.
// it will publish the states of all services to anyone listening on the internal states topic.
func (d *daemon) serviceStateWatcher(statePublishC chan<- States, stateUpdateC <-chan StateUpdate, stopC chan struct{}) <-chan struct{} {
	signalDoneC := make(chan struct{})

	go func() {
		defer close(signalDoneC)

		// create a local states map to track the state of each service.
		localStates := make(States)

		d.services.Range(func(anyName, anyService interface{}) bool {
			name, ok := anyName.(string)
			if !ok {
				d.log.Error("state watcher failed to update local service cache", "name", name, "error", "failed to cast service name to string")
				return false
			}
			// update the local states map with the initial state of each service.
			localStates[name] = InitState
			return true
		})

		for {
			select {
			case <-stopC:
				return
			case update, open := <-stateUpdateC:
				if !open {
					return
				}
				currState, found := localStates[update.Name]
				if found && currState == update.State {
					// skip any non-changes from previous state.
					continue
				}

				// update the local state
				localStates[update.Name] = update.State

				// make a copy of the updated local states to send out
				states := make(States)
				for name, state := range localStates {
					states[name] = state
				}

				// attempt to publish states to anyone still listening or exit if stop signal received.
				select {
				case <-stopC:
					return
				case statePublishC <- states:
				default:
					// no one was registered to receive the states, drop it.
				}
			}
		}
	}()

	return signalDoneC
}

// signal watcher is run in its own routine by daemon to watch for context done and OS Signals.
func (d *daemon) signalWatcher(wg *sync.WaitGroup, ctx context.Context, signalPublishC chan<- rxdSignal) {
	defer wg.Done()

	if len(d.conf.Signals) == 0 {
		d.conf.Signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	// Watch for OS Signals in separate go routine so we dont block main thread.
	d.log.Debug("daemon starting signal watcher")

	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, d.conf.Signals...)

	// TODO: future, add restart/hot-reload signal handler
	// after receiving any signal, inform manager to stop.
	defer func() {
		signal.Stop(signalC)         // stop receiving OS signals
		close(signalC)               // close the signal channel
		signalPublishC <- signalStop // signal to manager via intracom to stop
	}()

	for {
		select {
		case <-ctx.Done():
			d.log.Debug("daemon received context done signal")
			return
		case <-signalC:
			d.log.Debug("daemon os signal received")
			return
		}
	}
}
