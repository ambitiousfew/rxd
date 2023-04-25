package rxd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type daemon struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup

	manager *manager

	// logger *Logger
	logger Logging

	logCh chan LogMessage

	stopCh    chan struct{}
	stopLogCh chan struct{}
}

// SetLogSeverity allows for the logging to be scoped to severity level
func (d *daemon) SetLogger(logger Logging) {
	d.logger = logger
}

// Logger returns the instance of the daemon logger
func (d *daemon) Logger() Logging {
	return d.logger
}

// NewDaemon creates and return an instance of the reactive daemon
func NewDaemon(services ...*ServiceContext) *daemon {
	ctx, cancel := context.WithCancel(context.Background())

	// default severity to log is Info level and higher.
	logger := NewLogger(LevelDebug)

	logC := make(chan LogMessage, 10)

	manager := NewManager(services)
	manager.setLogCh(logC)

	return &daemon{
		ctx:     ctx,
		cancel:  cancel,
		wg:      new(sync.WaitGroup),
		manager: manager,
		logger:  logger,
		logCh:   logC,

		stopCh: make(chan struct{}),
		// stopLogCh is closed by daemon to signal to log watcher to stop.
		stopLogCh: make(chan struct{}),
	}
}

// Start the entrypoint for the reactive daemon. It launches 2 watcher routines.
//  1. Watching specifically for OS Signals which when received will inform the
//     manager to shutdown all services, blocks until finishes.
//  2. Log watcher that handles all logging from manager and services through a channel.
func (d *daemon) Start() (exitErr error) {
	defer func() {
		// capture any panics, convert to error to return
		if rErr := recover(); rErr != nil {
			exitErr = fmt.Errorf("%s", rErr)
		}
	}()

	d.wg.Add(3)
	// OS Signal watcher routine.
	go d.signalWatcher()
	// Logging routine.
	go d.logWatcher()

	// Run manager in its own thread so all wait using waitgroup
	go func() {
		defer func() {
			d.wg.Done()

			d.logger.Debug("daemon closing stopCh and stopLogCh")
			// signal stopping of daemon
			close(d.stopCh)
			// Signal stop of Logging routine
			close(d.stopLogCh)
		}()

		exitErr = d.manager.start() // Blocks main thread until all services stop to end wg.Wait() blocking.
		if exitErr != nil {
			d.logger.Error(exitErr.Error())
		}
		d.logger.Debug("manager routine ending")

	}()

	// Blocks the main thread, d.wg.Done() must finish all routines before we can continue beyond.
	d.wg.Wait()
	// close the logging channel - logC cannot be used after this point.
	close(d.logCh)
	d.logger.Debug("logging channel closed")
	return exitErr
}

func (d *daemon) AddService(service *ServiceContext) {
	d.manager.services = append(d.manager.services, service)
}

func (d *daemon) signalWatcher() {
	defer func() {
		// wait to hear from manager before returning
		// might still be sending messages.
		d.logger.Debug("signalWatcher waiting for manager to finish...")
		<-d.manager.ctx.Done()
		d.logger.Debug("signalWatcher manager stop signal received")

		d.wg.Done()
	}()

	signalC := make(chan os.Signal)
	// Watch for OS Signals in separate go routine so we dont block main thread.
	d.logger.Info("Daemon: starting OS signal watcher")

	signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-signalC:
			d.logger.Debug("OS signal received, cancelling context")
			// if we get an OS signal, we need to end.
			d.cancel()
			close(d.manager.stopCh)
			return
		case <-d.manager.ctx.Done():
			d.logger.Debug("manager context has been cancelled")
			// if manager ctx has been cancelled
			return
		case <-d.stopCh:
			// if manager completes we are done running...
			d.logger.Debug("daemon received stop signal")
			return
		}
	}
	// shutdown iterates over all services manager knows about signaling shutdown by closing the ShutdownC in each Service Config

}

func (d *daemon) logWatcher() {
	defer d.wg.Done()

	for {
		select {
		case <-d.stopLogCh:
			d.logger.Debug("stopping log watcher routine")
			return
		case logMsg := <-d.logCh:
			switch logMsg.Level {
			case Debug:
				d.logger.Debug(logMsg.Message)
			case Info:
				d.logger.Info(logMsg.Message)
			case Error:
				d.logger.Error(logMsg.Message)
			}
		}
	}
}
