package rxd

import (
	"fmt"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

// ServiceManager interface defines the methods that a service handler must implement
type ServiceManager interface {
	Manage(ctx ServiceContext, dService DaemonService, updateC chan<- StateUpdate)
}

type ManagerStateTimeouts map[State]time.Duration

// RunContinuousManager is a service handler that does its best to run the service
// moving the service to the next desired state returned from each lifecycle
// The handle will override the state transition if the context is cancelled
// and force the service to Exit.
type RunContinuousManager struct {
	DefaultDelay  time.Duration
	StartupDelay  time.Duration
	StateTimeouts ManagerStateTimeouts
}

func NewDefaultManager(opts ...ManagerOption) RunContinuousManager {
	timeouts := make(ManagerStateTimeouts)
	m := RunContinuousManager{
		StartupDelay:  10 * time.Nanosecond,
		StateTimeouts: timeouts,
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

// RunContinuousManager runs the service continuously until the context is cancelled.
// service contains the service runner that will be executed.
// which is then handled by the daemon.
func (m RunContinuousManager) Manage(sctx ServiceContext, ds DaemonService, updateC chan<- StateUpdate) {
	defer func() {
		// if any panics occur with the users defined service runner, recover and push error out to daemon logger.
		if r := recover(); r != nil {
			sctx.Log(log.LevelError, fmt.Sprintf("recovered from a panic: %v", r))
		}
	}()

	timeout := time.NewTimer(m.StartupDelay)
	defer timeout.Stop()

	// run continous manager will always start from the init state.
	var state State = StateInit

	var hasStopped bool

	for state != StateExit {
		// signal the current state we are about to enter. to the daemon states watcher.
		updateC <- StateUpdate{Name: ds.Name, State: state}

		select {
		case <-sctx.Done():
			// if the context is cancelled, transition to exit so we exit the loop.
			state = StateExit
			continue
		case <-timeout.C:
			if hasStopped {
				// if we enter are entering this block we are attempting a state other than exit.
				// reset hasStopped to false to ensure we don't skip stop after re-inits...
				hasStopped = false
			}

			switch state {
			case StateInit:
				if err := ds.Runner.Init(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					// if an error occurs in init state, transition to stop skipping idle and run.
					state = StateStop
				} else {
					// if no error occurs in init state, transition to idle.
					state = StateIdle
				}
			case StateIdle:
				if err := ds.Runner.Idle(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					// if an error occurs in idle state, transition to stop skipping run.
					state = StateStop
				} else {
					// if no error occurs in idle state, transition to run.
					state = StateRun
				}
			case StateRun:
				if err := ds.Runner.Run(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
				// run continous manager will always go back to stop after run to perform any cleanup.
				state = StateStop
			case StateStop:
				if err := ds.Runner.Stop(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
				// run continous manager will always go back to init after stop unless context is cancelled.
				state = StateInit
				// flip hasStopped to true to ensure we don't run stop again if Exit is next.
				hasStopped = true
			}

			// reset the timeout to the next desired state, if transition timeout not set use default.
			if transitionTimeout, ok := m.StateTimeouts[state]; ok {
				timeout.Reset(transitionTimeout)
			} else {
				timeout.Reset(m.DefaultDelay)
			}
		}
	}

	// once exiting the loop we are committed to exiting the service.
	// but we always want to ensure that the service has run stop proceeding
	if !hasStopped {
		err := ds.Runner.Stop(sctx)
		if err != nil {
			sctx.Log(log.LevelError, err.Error())
		}
	}

	// push final state to the daemon states watcher.
	updateC <- StateUpdate{Name: ds.Name, State: StateExit}
}

type RunUntilSuccessManager struct {
	StartupDelay time.Duration
	DefaultDelay time.Duration
}

// NewRunUntilSuccessManager creates a new RunUntilSuccessManager with the provided startup delay.
// RunUntilSuccessManager will continue to try to run the service lifecycles until the service
// exits Run with nil error.
func NewRunUntilSuccessManager(defaultDelay, startupDelay time.Duration) RunUntilSuccessManager {
	m := RunUntilSuccessManager{
		StartupDelay: startupDelay,
		DefaultDelay: defaultDelay,
	}

	return m
}

func (m RunUntilSuccessManager) Manage(sctx ServiceContext, ds DaemonService, updateC chan<- StateUpdate) {
	defer func() {
		// if any panics occur with the users defined service runner, recover and push error out to daemon logger.
		if r := recover(); r != nil {
			sctx.Log(log.LevelError, fmt.Sprintf("recovered from a panic: %v", r))
		}
	}()

	ticker := time.NewTicker(m.StartupDelay)
	defer ticker.Stop()

	var hasStopped bool
	// run continous manager will always start from the init state.
	var state State = StateInit
	select {
	case <-sctx.Done():
		state = StateExit
	case <-ticker.C:
		// startup delay has passed, we can start the service runner loop.
		if err := ds.Runner.Init(sctx); err != nil {
			sctx.Log(log.LevelError, err.Error())
			state = StateStop
		}
		state = StateIdle
		ticker.Reset(m.DefaultDelay)
	}

	for state != StateExit {
		// relay the current state we are about to enter to the daemon's states watcher.
		updateC <- StateUpdate{Name: ds.Name, State: state}

		select {
		case <-sctx.Done():
			// if the context is cancelled, transition to exit so we exit the loop.
			state = StateExit
			continue
		case <-ticker.C:
			if hasStopped {
				// if we enter are entering this block we are attempting a state other than exit.
				hasStopped = false
			}

			switch state {
			case StateInit:
				if err := ds.Runner.Init(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
					continue
				}
				state = StateIdle

			case StateIdle:
				if err := ds.Runner.Idle(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
					continue
				}
				state = StateRun

			case StateRun:
				if err := ds.Runner.Run(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
					continue
				}
				// run exited successfully, we can exit the loop.
				state = StateExit
			case StateStop:
				if err := ds.Runner.Stop(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
				state = StateInit
				hasStopped = true
			}
		}

	}

	if !hasStopped {
		// ensure that if any lifecycle ran after stop, we run stop again (for cleanup).
		if err := ds.Runner.Stop(sctx); err != nil {
			sctx.Log(log.LevelError, err.Error())
		}
	}

	// push final state to the daemon states watcher.
	updateC <- StateUpdate{Name: ds.Name, State: StateExit}

}
