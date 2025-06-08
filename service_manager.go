package rxd

import (
	"fmt"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

// ServiceManager interface defines the behavior of a service manager.
// It is responsible for managing the lifecycle of a service, including transitioning
// between different states such as Init, Idle, Run, Stop, and Exit.
type ServiceManager interface {
	Manage(ctx ServiceContext, dService DaemonService, updateC chan<- ServiceStateUpdate)
}

// ManagerStateTimeouts is a map of State to time.Duration.
// This is used to define the timeouts for each state transition in a service manager.
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

// NewDefaultManager creates a new RunContinuousManager with the provided options.
// The StartupDelay is used to delay the first run of the service after the manager is started.
// The DefaultDelay is used for all subsequent state transitions if not overridden in the StateTimeouts map.
func NewDefaultManager(opts ...ManagerOption) RunContinuousManager {
	timeouts := make(ManagerStateTimeouts)
	m := RunContinuousManager{
		DefaultDelay:  100 * time.Millisecond,
		StartupDelay:  100 * time.Millisecond,
		StateTimeouts: timeouts,
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

// Manage will run the service continuously until the context is cancelled.
// It will start from the init state, then idle, run, stop and back to init.
// The only condition that will cause the service to exit is if the context is cancelled.
// It is up to the caller to decide how to track and handle waiting between any state transitions beyond the defaults.
// The default state timeouts are used if not overridden in the StateTimeouts map.
func (m RunContinuousManager) Manage(sctx ServiceContext, ds DaemonService, updateC chan<- ServiceStateUpdate) {
	timeout := time.NewTimer(m.StartupDelay)
	defer timeout.Stop()

	// run continous manager will always start from the init state.
	var state = StateInit

	var hasStopped bool

	for state != StateExit {
		// signal the current state we are about to enter. to the daemon states watcher.
		updateC <- ServiceStateUpdate{Name: ds.Name, State: state, Transition: TransitionEntering}

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

				// signal the current state we have exited to the daemon states watcher.
				updateC <- ServiceStateUpdate{Name: ds.Name, State: StateInit, Transition: TransitionExited}
			case StateIdle:
				if err := ds.Runner.Idle(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					// if an error occurs in idle state, transition to stop skipping run.
					state = StateStop
				} else {
					// if no error occurs in idle state, transition to run.
					state = StateRun
				}
				// signal the current state we have exited to the daemon states watcher.
				updateC <- ServiceStateUpdate{Name: ds.Name, State: StateIdle, Transition: TransitionExited}

			case StateRun:
				if err := ds.Runner.Run(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
				// signal the current state we have exited to the daemon states watcher.
				updateC <- ServiceStateUpdate{Name: ds.Name, State: StateRun, Transition: TransitionExited}
				// run continous manager will always go back to stop after run to perform any cleanup.
				state = StateStop
			case StateStop:
				if err := ds.Runner.Stop(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
				// signal the current state we have exited to the daemon states watcher.
				updateC <- ServiceStateUpdate{Name: ds.Name, State: StateStop, Transition: TransitionExited}
				// run continous manager will always go back to init after stop unless context is cancelled.
				state = StateInit
				// flip hasStopped to true to ensure we don't run stop again if Exit is next.
				hasStopped = true
			default:
				sctx.Log(log.LevelError, "unknown state encountered", log.String("state", state.String()))
				continue
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
		// signal the current state we have exited to the daemon states watcher.
		updateC <- ServiceStateUpdate{Name: ds.Name, State: StateStop, Transition: TransitionEntering}
		err := ds.Runner.Stop(sctx)
		if err != nil {
			sctx.Log(log.LevelError, err.Error())
		}
		updateC <- ServiceStateUpdate{Name: ds.Name, State: StateStop, Transition: TransitionExited}
	}

	// push final state to the daemon states watcher.
	updateC <- ServiceStateUpdate{Name: ds.Name, State: StateExit, Transition: TransitionEntering}
}

// RunUntilSuccessManager represents a service manager that will run the service lifecycles
// until the service exits Run with nil error. It will retry the service lifecycles
// with a delay between attempts. The manager will start with a startup delay and then
// use the default delay for subsequent attempts. If the service stops, it will re-init
type RunUntilSuccessManager struct {
	StartupDelay time.Duration
	DefaultDelay time.Duration
}

// NewRunUntilSuccessManager creates a new RunUntilSuccessManager with the provided default and startup delays.
// The startup delay is used to delay the first run of the service after the manager is started.
// The default delay is used for subsequent runs after the service has stopped or exited.
func NewRunUntilSuccessManager(defaultDelay, startupDelay time.Duration) RunUntilSuccessManager {
	m := RunUntilSuccessManager{
		StartupDelay: startupDelay,
		DefaultDelay: defaultDelay,
	}

	return m
}

// Manage will run the service until it exits Run with a nil error.
// It will start with a startup delay, then run the service lifecycles in a loop.
// This manager is ideal for situation where you have a service that needs to succeed only a single time
// during its Run lifecycle.
// This manager usually indicates that the service isn't intended to run continuously for as long as the daemon is running,
// but rather it is expected to run until it succeeds and then exit. It can be use to perform one-off tasks
// or used to signal to other rxd services that it has completed its task before they move to their next state(s).
func (m RunUntilSuccessManager) Manage(sctx ServiceContext, ds DaemonService, updateC chan<- ServiceStateUpdate) {
	defer func() {
		// if any panics occur with the users defined service runner, recover and push error out to daemon logger.
		if r := recover(); r != nil {
			sctx.Log(log.LevelError, fmt.Sprintf("recovered from a panic: %v", r))
		}
	}()

	ticker := time.NewTicker(m.StartupDelay)
	defer ticker.Stop()

	var hasStopped bool

	// run until success will always start from the init state.
	var state = StateInit

	select {
	case <-sctx.Done():
		state = StateExit
	case <-ticker.C:
		updateC <- ServiceStateUpdate{Name: ds.Name, State: state, Transition: TransitionEntering}
		// startup delay has passed, we can start the service runner loop.
		if err := ds.Runner.Init(sctx); err != nil {
			sctx.Log(log.LevelError, err.Error())
			state = StateStop
		} else {
			state = StateIdle
		}

		// signal the init state we have exited to the daemon states watcher.
		updateC <- ServiceStateUpdate{Name: ds.Name, State: StateInit, Transition: TransitionExited}
		ticker.Reset(m.DefaultDelay)
	}

	for state != StateExit {
		updateC <- ServiceStateUpdate{Name: ds.Name, State: state, Transition: TransitionEntering}
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
				updateC <- ServiceStateUpdate{Name: ds.Name, State: StateInit, Transition: TransitionExited}
				state = StateIdle

			case StateIdle:
				if err := ds.Runner.Idle(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
					continue
				}
				updateC <- ServiceStateUpdate{Name: ds.Name, State: StateIdle, Transition: TransitionExited}
				state = StateRun

			case StateRun:
				if err := ds.Runner.Run(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
					continue
				}
				updateC <- ServiceStateUpdate{Name: ds.Name, State: StateRun, Transition: TransitionExited}

				// run exited successfully, we can exit the loop.
				state = StateExit
			case StateStop:
				// entering stop state, inform the daemon states watcher.
				if err := ds.Runner.Stop(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
				}

				// we have exited stop, inform the daemon states watcher.
				updateC <- ServiceStateUpdate{Name: ds.Name, State: StateStop, Transition: TransitionExited}

				state = StateInit
				hasStopped = true

			}
		}

	}

	if !hasStopped {
		updateC <- ServiceStateUpdate{Name: ds.Name, State: StateStop, Transition: TransitionEntering}
		// ensure that if any lifecycle ran after stop, we run stop again (for cleanup).
		if err := ds.Runner.Stop(sctx); err != nil {
			sctx.Log(log.LevelError, err.Error())
		}
		updateC <- ServiceStateUpdate{Name: ds.Name, State: StateStop, Transition: TransitionExited}
	}

	// push final state to the daemon states watcher, "entering" the exit state.
	updateC <- ServiceStateUpdate{Name: ds.Name, State: StateExit, Transition: TransitionEntering}

}
