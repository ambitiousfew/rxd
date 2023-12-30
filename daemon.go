package rxd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/ambitiousfew/intracom"
	"golang.org/x/exp/slog"
)

type daemon struct {
	services []*ServiceContext
	conf     DaemonConfig

	iSignals *intracom.Intracom[rxdSignal] // signals from daemon to manager
	iStates  *intracom.Intracom[States]    // services state updates to manager

	logger *slog.Logger

	// stopCh is used to signal to the signal watcher routine to stop.
	stopCh chan struct{}
	// stopLogCh is closed when daemon is exiting to stop the log watcher routine to stop.
	stopLogCh chan struct{}

	started bool
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
		services: []*ServiceContext{},
		conf:     conf,

		iSignals: iSignals,
		iStates:  iStates,

		logger: logger,
		// stopCh is closed by daemon to signal the signal watcher daemon wants to stop.
		stopCh: make(chan struct{}, 1),
		// stopLogCh
		stopLogCh: make(chan struct{}),

		started: false,
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
			d.logger.Error(err.Error())
			continue
		}
	}
	return nil
}

func (d *daemon) Start(ctx context.Context) error {
	if d.started {
		return fmt.Errorf("daemon is already running")
	}

	if len(d.services) < 1 {
		return fmt.Errorf("no services were added prior to starting the daemon")
	}

	// start an intracom instance for daemon to communicate with manager
	err := d.iSignals.Start()
	if err != nil {
		return err
	}
	defer d.iSignals.Close()

	// start an intracom instance for services to communicate with manager
	err = d.iStates.Start()
	if err != nil {
		return err
	}
	defer d.iStates.Close()

	d.started = true

	var wg sync.WaitGroup
	wg.Add(2) // signal watcher and manager

	go d.signalWatcher(&wg, ctx) // OS Signal watcher routine.
	go d.startManager(&wg)       // handles start and watching for signal to shutdown

	wg.Wait() // wait for signal watcher and manager to finish

	d.started = false
	d.logger.Debug("daemon is exiting")
	return nil
}

// addService is a helper function to add a service to the daemon.
func (d *daemon) addService(s *ServiceContext) error {
	if d.started {
		return fmt.Errorf("cannot add a service once the daemon is started")
	}

	for _, service := range d.services {
		if service.Name == s.Name {
			return fmt.Errorf("service name '%s' already exists", s.Name)
		}
	}
	d.services = append(d.services, s)
	return nil
}

func (d *daemon) startManager(wg *sync.WaitGroup) {
	defer wg.Done()

	// create manager instance
	manager := newManager(d.services)

	// ensure manager services do not have duplicate names
	err := manager.preStartCheck()
	if err != nil {
		d.logger.Error(err.Error())
		return
	}
	// set intracom instances for manager
	manager.iSignals = d.iSignals
	manager.iStates = d.iStates
	// set logger for manager and all services
	manager.log = d.logger

	go func() {
		// watch for signal from daemon to stop
		signalC, unsubscribe := d.iSignals.Subscribe(&intracom.SubscriberConfig{
			Topic:         internalSignalsManager,
			ConsumerGroup: "manager",
			BufferSize:    1,
			BufferPolicy:  intracom.DropNone,
		})
		defer unsubscribe()

		// TODO: in future we can handle specific signals here
		// such as restart/hot-reload of services without bringing down
		// the daemon and/or manager entirely.

		<-signalC          // wait for signal from daemon to stop
		manager.shutdown() // signal manager to begin shutdown, should unblock manager.start()
	}()

	manager.start() // blocks until all services started have stopped
}

func (d *daemon) signalWatcher(wg *sync.WaitGroup, ctx context.Context) {
	defer wg.Done()
	defer close(d.stopLogCh)

	signalManager, unregister := d.iSignals.Register(internalSignalsManager)
	defer unregister()

	if len(d.conf.Signals) == 0 {
		d.conf.Signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	// Watch for OS Signals in separate go routine so we dont block main thread.
	d.logger.Debug("daemon starting system signal watcher")

	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, d.conf.Signals...)

	var signaled bool

	for !signaled {
		select {
		case <-ctx.Done():
			d.logger.Debug("daemon received context done signal")
			signaled = true
		case <-signalC:
			d.logger.Debug("daemon os signal received")
			signaled = true
		case <-d.stopCh:
			// if manager completes we are done running...
			d.logger.Debug("daemon received stop signal")
			signaled = true
		}
	}

	signal.Stop(signalC)
	close(signalC)

	// TODO: future, add restart/hot-reload signal handler
	// after receiving any signal, inform manager to stop.
	signalManager <- signalStop
}
