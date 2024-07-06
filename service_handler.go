package rxd

import (
	"time"

	"github.com/ambitiousfew/rxd/log"
)

// ServiceHandler interface defines the methods that a service handler must implement
type ServiceHandler interface {
	Handle(ctx ServiceContext, runner ServiceRunner)
}

// DefaultHandler is a service handler that runs the service continuously
// either until an OS signal or the daemon Start context is cancelled.
// This is the default handler for a service unless specified otherwise by overriding with
// the UsingHandler service option.
type DefaultHandler struct{}

// Handle runs the service continuously until the context is cancelled.
// service contains the service runner that will be executed.
// errC is a channel that is used to report errors that occur during the service execution.
// which is then handled by the daemon.
func (h DefaultHandler) Handle(sctx ServiceContext, sr ServiceRunner) {
	var hasStopped bool
	var state State

	// Set the default timeout to 0 to default resets on everything else except stop.
	defaultTimeout := 0 * time.Nanosecond

	timeout := time.NewTimer(defaultTimeout)
	defer timeout.Stop()

	for state != StateExit {
		select {
		case <-sctx.Done():
			state = StateExit
		case <-timeout.C:
			switch state {
			case StateInit:
				state = StateIdle
				err := sr.Init(sctx)
				if err != nil {
					// errC <- err
					sctx.Log(log.LevelError, err.Error())
					state = StateExit // or Stop with timeout?
				}
			case StateIdle:
				state = StateRun
				err := sr.Idle(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
				}
			case StateRun:
				state = StateStop
				err := sr.Run(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
			case StateStop:
				state = StateInit
				err := sr.Stop(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
				}

				// TODO: Add a timeout to the stop state to prevent
				// the service from cycling too quickly between retries.
				hasStopped = true

				// TODO: This would be configurable via the service options
				timeout.Reset(3 * time.Second)
				continue // skip the default timeout reset
			}

			// fallback to the default timeout duration if we make it here.
			timeout.Reset(defaultTimeout)
		}
	}

	if !hasStopped {
		err := sr.Stop(sctx)
		if err != nil {
			sctx.Log(log.LevelError, err.Error())
		}
	}

}
