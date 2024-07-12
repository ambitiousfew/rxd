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

var DefaultHandler = RunContinuousHandler{}

// DefaultHandler is a service handler that runs the service continuously
// either until an OS signal or the daemon Start context is cancelled.
// This is the default handler for a service unless specified otherwise by overriding with
// the UsingHandler service option.
type RunContinuousHandler struct{}

// Handle runs the service continuously until the context is cancelled.
// service contains the service runner that will be executed.
// which is then handled by the daemon.
func (h RunContinuousHandler) Handle(sctx ServiceContext, ds DaemonService, updateState chan<- StateUpdate) {
	defer func() {
		// if any panics occur with the users defined service runner, recover and push error out to daemon logger.
		if r := recover(); r != nil {
			sctx.Log(log.LevelError, fmt.Sprintf("recovered from a panic: %v", r))
		}
	}()

	serviceName := sctx.Name()

	defaultTimeout := 10 * time.Nanosecond

	timeout := time.NewTimer(ds.TransitionTimeouts.GetOrDefault(TransExitToInit, defaultTimeout))
	defer timeout.Stop()

	var state State = StateInit

	var hasStopped bool
	for state != StateExit {
		// signal the current state we are about to enter. to the daemon states watcher.
		updateState <- StateUpdate{State: state, Name: serviceName}

		select {
		case <-sctx.Done():
			state = StateExit
		case <-timeout.C:
			switch state {
			case StateInit:
				state = StateIdle
				err := ds.Runner.Init(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateInit
					// reset the transition timeout to the init state
					timeout.Reset(ds.TransitionTimeouts.GetOrDefault(TransInitToIdle, defaultTimeout))
				}
				// reset the hasStopped flag since we just restarted the service.
				hasStopped = false

			case StateIdle:
				state = StateRun
				err := ds.Runner.Idle(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
					// reset the transition timeout to the init state
					timeout.Reset(ds.TransitionTimeouts.GetOrDefault(TransIdleToRun, defaultTimeout))

				}
			case StateRun:
				state = StateStop
				err := ds.Runner.Run(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
				// reset the transition timeout to the init state
				timeout.Reset(ds.TransitionTimeouts.GetOrDefault(TransRunToStop, defaultTimeout))
			case StateStop:
				err := ds.Runner.Stop(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
				// flip hasStopped to true since we just stopped the service.
				// to cover the case where ctx can be cancelled and always want to
				// ensure stop is called prior to StateExit to ensure cleanup.
				hasStopped = true

				// stop never leads to exit, always back to init for run continuous.
				state = StateInit

				// reset using the stop to init timeout if it exists.
				timeout.Reset(ds.TransitionTimeouts.GetOrDefault(TransStopToInit, defaultTimeout))
				continue // skip the default timeout reset
			}

			// fallback to the default timeout duration if we make it here.
			timeout.Reset(defaultTimeout)
		}
	}

	if !hasStopped {
		err := ds.Runner.Stop(sctx)
		if err != nil {
			sctx.Log(log.LevelError, err.Error())
		}
	}

}
