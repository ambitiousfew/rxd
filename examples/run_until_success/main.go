// For this example, we will create a service that will run until it is successful.
// The service will simulate a startup job that fails 3 times before it is successful.
// There is a startup delay of 3 seconds before the service enters the Init state.
// There is a transition delay of 1 second between each state transition after startup.
//
// Any errors during the service running will result in lifecycle changes that bring it back
// through to Run again. The first time Run exits with a non-nil error, the service will be
// transitioned to the Exit state by the RunUntilSuccessManager.

package main

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
)

const DaemonName = "single-service"

// StartupJobService must meet Service interface or line below errors.
var _ rxd.ServiceRunner = (*StartupJobService)(nil)

func main() {
	// NOTE: Intentional cancellation timeout after 30 seconds to show startup/shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// create a logging instance with a default or custom handler.
	logger := log.NewLogger(log.LevelInfo, log.NewHandler())

	startupJob := NewStartupJobService()
	// Use the RunUntilSuccessManager to run the service until it is successful.
	transitionDelay := 1 * time.Second
	startupDelay := 3 * time.Second
	manager := rxd.NewRunUntilSuccessManager(transitionDelay, startupDelay)

	// Give the service a name, the service runner, and any options you want to pass to the service such as manager.
	apiSvc := rxd.NewService("startup-job", startupJob, rxd.WithManager(manager))

	// configure any daemon options
	dopts := []rxd.DaemonOption{
		rxd.WithSignals(os.Interrupt, syscall.SIGINT, syscall.SIGTERM),
	}

	// Create a new daemon giving it a name, service logger and options
	daemon := rxd.NewDaemon(DaemonName, dopts...)

	// Add the single service to the daemon
	err := daemon.AddService(apiSvc)
	if err != nil {
		logger.Log(log.LevelError, err.Error())
		os.Exit(1)
	}

	// Start the daemon, this will block until the daemon is stopped via ctx cancel or OS signal.
	err = daemon.Start(ctx)
	if err != nil {
		logger.Log(log.LevelError, err.Error())
		os.Exit(1)
	}

	logger.Log(log.LevelInfo, "successfully stopped daemon")
}

type StartupJobService struct {
	attempts int
}

// NewHelloWorldService just a factory helper function to help create and return a new instance of the service.
func NewStartupJobService() *StartupJobService {
	return &StartupJobService{
		attempts: 0, // fake attempts to simulate a startup job that fails
	}
}

func (s *StartupJobService) Run(sctx rxd.ServiceContext) error {
	sctx.Log(log.LevelInfo, "entering run")
	select {
	case <-sctx.Done():
		return nil
	default:
		sctx.Log(log.LevelInfo, "performing work")

		if s.attempts < 3 {
			s.attempts++
			return errors.New("failed to run startup job")
		}

		// after 3 attempts, we are successful
		return nil
	}
}

func (s *StartupJobService) Init(sctx rxd.ServiceContext) error {
	sctx.Log(log.LevelInfo, "entering init")
	select {
	case <-sctx.Done():
		return nil
	default:
		return nil
	}
}

func (s *StartupJobService) Idle(sctx rxd.ServiceContext) error {
	// Perform any idling work here. This is a good place to wait for signals or other events.
	// Retry connections, wait for other services to enter a given state, etc.
	sctx.Log(log.LevelInfo, "entering idle")
	select {
	case <-sctx.Done():
		return nil
	default:
		return nil
	}
}

func (s *StartupJobService) Stop(sctx rxd.ServiceContext) error {
	sctx.Log(log.LevelInfo, "entering stop")
	// perform any cleanup that idle or run may have created.
	select {
	case <-sctx.Done():
		return nil
	default:
		return nil
	}
}
