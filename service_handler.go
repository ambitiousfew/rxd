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

type HandlerStateTimeouts map[State]time.Duration

// DefaultHandler is a service handler that does its best to run the service
// moving the service to the next desired state returned from each lifecycle
// The handle will override the state transition if the context is cancelled
// and force the service to Exit.
type DefaultHandler struct {
	StartupDelay  time.Duration
	StateTimeouts HandlerStateTimeouts
}

func NewDefaultHandler(opts ...HandlerOption) DefaultHandler {
	timeouts := make(HandlerStateTimeouts)
	h := DefaultHandler{
		StartupDelay:  10 * time.Nanosecond,
		StateTimeouts: timeouts,
	}

	for _, opt := range opts {
		opt(&h)
	}

	return h
}

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

	// default timeout for state all transitions if not set and excluding StartDelay which applies to first time Init.
	defaultTimeout := 10 * time.Nanosecond

	timeout := time.NewTimer(h.StartupDelay)
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
			state = StateExit
			continue
		case <-timeout.C:
			switch state {
			case StateInit:
				state, err = ds.Runner.Init(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateInit
					// reset the transition timeout to the init state
				}
				// reset the hasStopped flag since we just restarted the service.
				hasStopped = false

			case StateIdle:
				state, err = ds.Runner.Idle(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
				}
			case StateRun:
				state, err = ds.Runner.Run(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
				}

			case StateStop:
				state, err = ds.Runner.Stop(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
				}

				// flip hasStopped to true to ensure we don't run stop again if Exit is next.
				hasStopped = true
				continue // skip the default timeout reset
			}

			// reset the timeout to the next desired state, if transition timeout not set use default.
			if transitionTimeout, ok := h.StateTimeouts[state]; ok {
				timeout.Reset(transitionTimeout)
			} else {
				timeout.Reset(defaultTimeout)
			}
		}
	}

	// ensure stop is always run before exit, if it hasn't been run already.
	if !hasStopped {
		_, err := ds.Runner.Stop(sctx)
		if err != nil {
			sctx.Log(log.LevelError, err.Error())
		}
	}

	// push final state to the daemon states watcher.
	updateState <- StateUpdate{State: StateExit, Name: ds.Name}
}
