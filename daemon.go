package rxd

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type daemon struct {
	wg *sync.WaitGroup

	// manager handles all service related operations: context wrapper, state changes, notifiers
	manager *manager

	// logger *Logger
	logger Logging

	// stopCh is used to signal to the signal watcher routine to stop.
	stopCh chan struct{}
	// stopLogCh is closed when daemon is exiting to stop the log watcher routine to stop.
	stopLogCh chan struct{}
}

// SetCustomLogger set a custom logger that meets logging interface for the daemon to use.
func (d *daemon) SetCustomLogger(logger Logging) {
	d.logger = logger
	d.manager.setLogger(logger)
}

// SetDefaultLogger allows for default logger to be defined with customized logging flags.
func (d *daemon) SetDefaultLogger(level LogSeverity, flags int) {
	d.logger = NewLogger(level, flags)
}

// Logger returns the instance of the daemon logger
func (d *daemon) Logger() Logging {
	return d.logger
}

// NewDaemon creates and return an instance of the reactive daemon
func NewDaemon(services ...*ServiceContext) *daemon {
	// default severity to log is Info level and higher.
	logger := NewLogger(LevelInfo, NoFlags)

	manager := newManager(services)
	manager.setLogger(logger)

	return &daemon{
		wg:      new(sync.WaitGroup),
		manager: manager,
		logger:  logger,

		// stopCh is closed by daemon to signal the signal watcher daemon wants to stop.
		stopCh: make(chan struct{}),
		// stopLogCh
		stopLogCh: make(chan struct{}),
	}
}

// Start the entrypoint for the reactive daemon. It launches 3 routines for its wait group.
//  1. Watching specifically for OS Signals which when received will inform the
//     manager to shutdown all services, blocks until finishes.
//  2. Log watcher that handles all logging from manager and services through a channel.
//  3. Manager routine to handle running and managing services.
func (d *daemon) Start() error {
	var err error
	if len(d.manager.services) < 1 {
		return fmt.Errorf("no services were provided to rxd daemon to start")
	}

	d.wg.Add(3)
	// OS Signal watcher routine.
	go d.signalWatcher()

	// Add the managers stateUpdate watcher
	stateWatcherStopC := make(chan struct{})
	go func() {
		// manager service states watcher routine.
		// needs to be outside manager start waitgroup
		// since we are trying to stop this after start is done.
		defer d.wg.Done()
		d.manager.serviceStateWatcher(stateWatcherStopC)
	}()

	// Run manager in its own thread so all wait using waitgroup
	go func() {
		defer func() {
			d.logger.Debug("rxdaemon closing stopCh and stopLogCh")
			// when manager start is done, stop serviceStateWatcher
			close(stateWatcherStopC)
			// signal stopping of daemon
			close(d.stopCh)
			d.wg.Done()
		}()

		err = d.manager.preStartCheck()
		if err != nil {
			return
		}

		// Blocks main thread until all services stop to end wg.Wait() blocking.
		err = d.manager.start()
	}()
	// Blocks the main thread, d.wg.Done() must finish all routines before we can continue beyond.
	d.wg.Wait()

	d.logger.Debug("rxdaemon is exiting")
	return err
}

func (d *daemon) AddService(service *ServiceContext) {
	d.manager.services = append(d.manager.services, service)
}

func (d *daemon) signalWatcher() {
	// Watch for OS Signals in separate go routine so we dont block main thread.
	d.logger.Debug("rxdaemon starting system signal watcher")

	defer func() {
		// wait to hear from manager before returning
		// might still be sending messages.
		d.manager.shutdown()
		d.logger.Debug("rxdaemon signal watcher waiting for manager to finish shutting down...")
		<-d.manager.ctx.Done()
		d.logger.Debug("rxdaemon signal watcher manager received done signal from manager shutdown")
		// wait for signal that manager exited start()
		<-d.manager.stopCh
		d.logger.Debug("rxdaemon signal watcher received signal that manager has exited start")
		// logging routine stays open until manager signals it finished running start().
		// Signal stop of Logging routine
		close(d.stopLogCh)

		d.wg.Done()
	}()

	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-signalC:
			d.logger.Debug("rxdaemon os signal received, cancelling context")
			return
		case <-d.stopCh:
			// if manager completes we are done running...
			d.logger.Debug("rxdaemon received stop signal")
			return
		}
	}
}
