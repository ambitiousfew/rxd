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
	intracom *intracom.Intracom[[]byte]

	services []*ServiceContext

	log    *slog.Logger
	stopCh chan struct{}
}

// NewDaemon creates and return an instance of the reactive daemon
func NewDaemon(services ...*ServiceContext) *daemon {
	// default severity to log is Info level and higher.
	logger := slog.Default()
	ic := intracom.New[[]byte]()

	return &daemon{
		services: services,
		intracom: ic,
		log:      logger,
		stopCh:   make(chan struct{}),
	}
}

// SetIntracom set an intracom instance to be used by the daemon, manager, and services.
// intracom is used for intra-service communication ("within" the rxd daemon).
// A new default intracom instance is created if one is not provided.
// NOTE: This must be called before Start() is called.
func (d *daemon) SetIntracom(i *intracom.Intracom[[]byte]) {
	d.intracom = i
}

// SetLogHandler recreates the logger with the passed in handler.
func (d *daemon) SetLogHandler(handler slog.Handler) {
	d.log = slog.New(handler)
}

// Start the entrypoint for the reactive daemon. It launches 2 routines for its wait group.
//  1. Watches for context cancel or OS signals to trigger manager shutdown.
//  2. Manager routine to handle running and managing services.
//
// Blocks until all routines have finished.
// NOTE: This is a blocking call, it will not return until all services have completed.
func (d *daemon) Start(ctx context.Context) error {
	if len(d.services) < 1 {
		return fmt.Errorf("no services were provided to rxd daemon to start")
	}

	// Start the intracom comms bus
	if err := d.intracom.Start(); err != nil {
		return err
	}

	defer d.intracom.Close()

	manager := &manager{
		services: d.services,
		// N services * 2 (possible states)
		stateUpdateC: make(chan StateUpdate, len(d.services)*2),
		// intercom will be passed to each service to share for inter-service comms
		intracom:       d.intracom,
		log:            d.log,
		shutdownCalled: atomic.Int32{},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		// watch for both context and OS signals, whichever comes first.
		defer wg.Done()

		signalC := make(chan os.Signal, 1)

		defer close(signalC)
		defer manager.shutdown() // signal manager to shutdown all services in both cases
		defer signal.Stop(signalC)

		signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-ctx.Done():
			d.log.Debug("rxd daemon received context done signal")
			return
		case <-signalC:
			d.log.Debug("rxd daemon received OS signal")
			return
		}
	}()

	// Run manager in its own thread so all wait using waitgroup
	go func() {
		defer wg.Done()

		// preStartCheck ensures no duplicate service names were passed to daemon
		if err := manager.preStartCheck(); err != nil {
			// TODO: get errors out to daemon start
			return
		}

		manager.start() // blocks until all services are done running their lifecycles
	}()

	wg.Wait() // blocks until all routines are done

	d.log.Debug("rxd daemon is exiting")
	return nil
}
