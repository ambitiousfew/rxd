package rxd

import (
	"os"
	"os/signal"
	"syscall"
)

type daemon struct {
	logger   *Logger
	manager  *manager
	stoppedC chan struct{}
	logC     chan LogMessage
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
func NewDaemon(services ...Service) *daemon {
	// default severity to log is Info level and higher.
	logger := NewLogger(LevelInfo)

	logC := make(chan LogMessage)
	stopC := make(chan struct{})

	return &daemon{
		logger:   logger,
		stoppedC: stopC,
		logC:     logC,
		manager: &manager{
			Services: services,
			logC:     logC,
			stoppedC: stopC,
		},
	}
}

func (d *daemon) signalWatcher(signalC chan os.Signal) {
	stopLogC := make(chan struct{})

	defer func() {
		// wait to hear from manager before returning
		// might still be sending messages.
		d.logger.Debug.Println("Waiting for manager to finish...")
		<-d.stoppedC
		d.logger.Debug.Println("Manager stop signal received")
		close(stopLogC)
		d.logger.Debug.Println("stopLogC channel closed")
	}()

	// Watch for OS Signals in separate go routine so we dont block main thread.
	d.logger.Info.Println("Daemon: starting OS signal watcher")

	signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)

	<-signalC // blocks until a signal is received
	d.logger.Debug.Println("OS signal received, cancelling context")

	err := d.manager.shutdown()
	if err != nil {
		d.logger.Error.Println(err)
	}
}

func (d *daemon) logWatcher(stopLogC chan struct{}) {
	for {
		select {
		case <-stopLogC:
			d.logger.Debug.Println("stopping log watcher routine")
			return
		case logMsg := <-d.logC:
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

// Start the entrypoint for the reactive daemon. It launches 2 watcher routines.
//  1. Watching specifically for OS Signals which when received will inform the
//     manager to shutdown all services, blocks until finishes.
//  2. Log watcher that handles all logging from manager and services through a channel.
func (d *daemon) Start() error {
	stopLogC := make(chan struct{})
	signalC := make(chan os.Signal)

	defer func() {
		d.logger.Debug.Println("closing signalC channel")
		close(signalC)
		d.logger.Debug.Println("signalC channel closed")
		close(d.logC)
		d.logger.Debug.Println("logC channel closed")
	}()

	// OS Signal watcher routine.
	go d.signalWatcher(signalC)
	// Logging routine.
	go d.logWatcher(stopLogC)

	// Main thread blocks here until manager stops all services
	// which can be triggered by the relaying of OS Signal / context.Done()
	err := d.manager.Start()
	if err != nil {
		d.logger.Error.Println(err)
	}

	return err
}
