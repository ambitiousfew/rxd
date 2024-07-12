package rxd

import (
	"fmt"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

// ServiceHandler interface defines the methods that a service handler must implement
type ServiceHandler interface {
	Handle(ctx ServiceContext, dService DaemonService, stateUpdateC chan<- StateUpdate)
}

// DefaultHandler is a service handler that does its best to run the service
// moving the service to the next desired state returned from each lifecycle
// The handle will override the state transition if the context is cancelled
// and force the service to Exit.
type DefaultHandler struct{}

// Handle runs the service continuously until the context is cancelled.
// service contains the service runner that will be executed.
// which is then handled by the daemon.
func (h DefaultHandler) Handle(sctx ServiceContext, ds DaemonService, updateState chan<- StateUpdate) {
	defer func() {
		// if any panics occur with the users defined service runner, recover and push error out to daemon logger.
		if r := recover(); r != nil {
			sctx.Log(log.LevelError, fmt.Sprintf("recovered from a panic: %v", r))
		}
	}()

	defaultTimeout := 10 * time.Nanosecond

	timeout := time.NewTimer(ds.TransitionTimeouts.GetOrDefault(TransExitToInit, defaultTimeout))
	defer timeout.Stop()

	var state State = StateInit

	var hasStopped bool

	for state != StateExit {
		var err error
		// signal the current state we are about to enter. to the daemon states watcher.
		updateState <- StateUpdate{State: state, Name: ds.Name}

		if hasStopped {
			// not entering Exit after stop was true, reset hasStopped
			hasStopped = false
		}

		select {
		case <-sctx.Done():
			// push to exit.
			state = StateExit
		case <-timeout.C:
			switch state {
			case StateInit:
				state, err = ds.Runner.Init(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateInit
					// reset the transition timeout to the init state
					timeout.Reset(ds.TransitionTimeouts.GetOrDefault(TransInitToIdle, defaultTimeout))
				}
				// reset the hasStopped flag since we just restarted the service.
				hasStopped = false

			case StateIdle:
				state, err = ds.Runner.Idle(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
					// reset the transition timeout to the init state
					timeout.Reset(ds.TransitionTimeouts.GetOrDefault(TransIdleToRun, defaultTimeout))

				}
			case StateRun:
				state, err = ds.Runner.Run(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
				// reset the transition timeout to the init state
				timeout.Reset(ds.TransitionTimeouts.GetOrDefault(TransRunToStop, defaultTimeout))

			case StateStop:
				state, err = ds.Runner.Stop(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
				}

				// flip hasStopped to true to ensure we don't run stop again if Exit is next.
				hasStopped = true

				// reset using the stop to init timeout if it exists.
				timeout.Reset(ds.TransitionTimeouts.GetOrDefault(TransStopToInit, defaultTimeout))
				continue // skip the default timeout reset
			}

			// fallback to the default timeout duration if we make it here.
			timeout.Reset(defaultTimeout)
		}
	}

	if !hasStopped {
		// we are only wanting to ensure stop is run if it hasn't been run already then exit.
		_, err := ds.Runner.Stop(sctx)
		if err != nil {
			sctx.Log(log.LevelError, err.Error())
		}
	}

	// push final state to the daemon states watcher.
	updateState <- StateUpdate{State: StateExit, Name: ds.Name}
}
