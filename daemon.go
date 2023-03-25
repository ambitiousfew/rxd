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

	logger *Logger
	logCh  chan LogMessage

	stopCh    chan struct{}
	stopLogCh chan struct{}
}

// SetLogSeverity allows for the logging to be scoped to severity level
func (d *daemon) SetLogSeverity(level LogSeverity) {
	d.logger = NewLogger(level)
}

// Logger returns the instance of the daemon logger
func (d *daemon) Logger() *Logger {
	return d.logger
}

// NewDaemon creates and return an instance of the reactive daemon
func NewDaemon(services ...*Service) *daemon {
	ctx, cancel := context.WithCancel(context.Background())

	// default severity to log is Info level and higher.
	logger := NewLogger(LevelInfo)

	logC := make(chan LogMessage, 10)

	return &daemon{
		ctx:    ctx,
		cancel: cancel,
		wg:     new(sync.WaitGroup),
		manager: &manager{
			ctx:      ctx,
			wg:       new(sync.WaitGroup),
			services: services,
			logC:     logC,
			// stopCh is closed by daemon to signal to manager to stop services
			stopCh: make(chan struct{}),
			// completeCh is closed by manager to signal to daemon it has finished - not sure its necessary
			completeCh: make(chan struct{}),
		},

		logger: logger,
		logCh:  logC,

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

	d.wg.Add(2)
	// OS Signal watcher routine.
	go d.signalWatcher()
	// Logging routine.
	go d.logWatcher()

	// Main thread blocks here until manager stops all services
	// which can be triggered by the relaying of OS Signal / context.Done()
	exitErr = d.manager.start() // Blocks main thread until all services stop to end wg.Wait() blocking.
	if exitErr != nil {
		d.logger.Error.Println(exitErr)
	}

	// signal stopping of daemon
	close(d.stopCh)
	// Signal stop of Logging routine
	close(d.stopLogCh)

	d.wg.Wait()

	// close the logging channel - logC cannot be used after this point.
	close(d.logCh)
	d.logger.Debug.Println("logging channel closed")
	return exitErr
}

func (d *daemon) AddService(service *Service) {
	d.manager.services = append(d.manager.services, service)
}

func (d *daemon) signalWatcher() {
	defer func() {
		// wait to hear from manager before returning
		// might still be sending messages.
		d.logger.Debug.Println("Waiting for manager to finish...")
		<-d.manager.completeCh
		d.logger.Debug.Println("Manager stop signal received")

		d.wg.Done()
	}()

	signalC := make(chan os.Signal)
	// Watch for OS Signals in separate go routine so we dont block main thread.
	d.logger.Info.Println("Daemon: starting OS signal watcher")

	signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-signalC:
			d.logger.Debug.Println("OS signal received, cancelling context")
			// if we get an OS signal, we need to end.
			d.cancel()
			close(d.manager.stopCh)
			return
		case <-d.stopCh:
			// if manager completes we are done running...
			d.logger.Debug.Println("daemon received stop signal")
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
			d.logger.Debug.Println("stopping log watcher routine")
			return
		case logMsg := <-d.logCh:
			switch logMsg.Level {
			case Debug:
				d.logger.Debug.Println(logMsg.Message)
			case Info:
				d.logger.Info.Println(logMsg.Message)
			case Error:
				d.logger.Error.Println(logMsg.Message)
			}
		}
	}
}
