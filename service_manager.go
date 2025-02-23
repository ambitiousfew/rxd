package rxd

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ambitiousfew/rxd/log"
	"github.com/ambitiousfew/rxd/pkg/rpc"
)

// ServiceManager interface defines the methods that a service handler must implement
type ServiceManager interface {
	Manage(parent context.Context, dService DaemonService, updateC chan<- StateUpdate)
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
		DefaultDelay:  100 * time.Millisecond,
		StartupDelay:  100 * time.Millisecond,
		StateTimeouts: timeouts,
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

func (m RunContinuousManager) nextState(currentState State, signal rpc.CommandSignal) (State, error) {
	switch signal {
	case rpc.CommandSignalStart:
		if currentState == StateExit || currentState == StateStop {
			return StateInit, nil
		}
		return currentState, fmt.Errorf("cannot start from state: %v", currentState)
	case rpc.CommandSignalStop:
		if currentState != StateExit {
			return StateExit, nil
		}
		return currentState, fmt.Errorf("cannot stop from state: %v", currentState)
	case rpc.CommandSignalRestart:
		if currentState != StateExit {
			return StateStop, nil
		}
		return currentState, fmt.Errorf("cannot restart from state: %v", currentState)
	case rpc.CommandSignalReload:
		if currentState != StateExit {
			return StateStop, nil
		}
		return currentState, fmt.Errorf("cannot reload from state: %v", currentState)
	default:
		return currentState, fmt.Errorf("unknown command signal: %v", signal)
	}
}

func (m RunContinuousManager) runService(sctx ServiceContext, ds DaemonService, updateC chan<- StateUpdate) <-chan struct{} {
	doneC := make(chan struct{})

	go func(sctx ServiceContext, ds DaemonService) {
		defer close(doneC)

		var state State = StateInit
		var hasStopped bool

		timeout := time.NewTimer(m.StartupDelay)
		defer timeout.Stop()

	loop:
		for {
			select {
			case <-sctx.Done():
				break loop
			case <-timeout.C:
				if hasStopped && state != StateExit {
					hasStopped = false
				}

				// push the current state to the daemon states watcher.
				updateC <- StateUpdate{Name: ds.Name, State: state}

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
					}
					state = StateStop
				case StateStop:
					if err := ds.Runner.Stop(sctx); err != nil {
						sctx.Log(log.LevelError, err.Error())
					}
					hasStopped = true

					state = StateInit
				default:
					state = StateExit
				}

				// delay between each state transition
				delay, found := m.StateTimeouts[state]
				if !found {
					delay = m.DefaultDelay
				}
				timeout.Reset(delay)
			}
		}

		if !hasStopped {
			timedCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			stopCtx := sctx.WithParent(timedCtx)

			sctx, cancel := NewServiceContextWithCancel(stopCtx, ds)
			defer cancel()

			if err := ds.Runner.Stop(sctx); err != nil {
				sctx.Log(log.LevelError, err.Error())
			}
		}

		updateC <- StateUpdate{Name: ds.Name, State: StateExit}

	}(sctx, ds)

	return doneC
}

type CommandRequest struct {
	Signal rpc.CommandSignal
	Err    error
}

// friendlyTimeDiff formats a time difference in a human-readable way
func friendlyTimeDiff(elapsed time.Duration) string {
	switch {
	case elapsed < time.Minute:
		return fmt.Sprintf("%d seconds ago", int(elapsed.Seconds()))
	case elapsed < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(elapsed.Hours()))
	case elapsed < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(elapsed.Hours()/24))
	case elapsed < 30*24*time.Hour:
		return fmt.Sprintf("%d weeks ago", int(elapsed.Hours()/168)) // 168 hours in a week
	case elapsed < 365*24*time.Hour:
		return fmt.Sprintf("%d months ago", int(elapsed.Hours()/730)) // Approximate months
	default:
		return fmt.Sprintf("%d years ago", int(elapsed.Hours()/8760)) // Approximate years
	}
}

// RunContinuousManager runs the service continuously until the context is cancelled.
// service contains the service runner that will be executed.
// which is then handled by the daemon.
func (m RunContinuousManager) Manage(ctx context.Context, ds DaemonService, updateC chan<- StateUpdate) {
	var state State = StateInit
	mu := new(sync.RWMutex)

	signalC := make(chan CommandRequest, 1)

	go func() {
		defer close(signalC)
		// command relay routine
		// listens for rpc commands to change the state of the service.
		for {
			select {
			case <-ctx.Done(): // daemon wants to shut down
				return
			case cmd := <-ds.CommandC:
				mu.RLock()
				nextState, err := m.nextState(state, cmd)
				mu.RUnlock()
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case signalC <- CommandRequest{Err: err}:
					}
					continue
				}

				mu.Lock()
				state = nextState
				mu.Unlock()

				select {
				case <-ctx.Done():
					return
				case signalC <- CommandRequest{Signal: cmd}:
				}
			}
		}
	}()

	warningDuration := 10 * time.Second
	warningTimer := time.NewTimer(warningDuration)
	defer warningTimer.Stop()

	var stoppedAt *time.Time

	sctx, cancel := NewServiceContextWithCancel(ctx, ds)
	defer cancel()
	doneC := m.runService(sctx, ds, updateC)

loop:
	for {
		now := time.Now()
		select {
		case <-ctx.Done():
			cancel()
			break loop
		case <-warningTimer.C:
			if stoppedAt == nil {
				// stopped wasnt set
				continue
			}

			timeDiff := time.Since(*stoppedAt)
			sctx.Log(log.LevelWarning, "service has been stopped for over a minute", log.String("duration", friendlyTimeDiff(timeDiff)))
			warningTimer.Reset(warningDuration)
		case cmd, open := <-signalC:
			if !open {
				break loop
			}

			// command relay couldnt determine the next state.
			// push the error to the daemon logger.
			if cmd.Err != nil {
				sctx.Log(log.LevelError, cmd.Err.Error())
				continue
			}

			switch cmd.Signal {
			case rpc.CommandSignalStart:
				sctx, cancel = NewServiceContextWithCancel(ctx, ds)
				doneC = m.runService(sctx, ds, updateC)
				stoppedAt = nil
				// do nothing, the service is already running.
			case rpc.CommandSignalStop:
				cancel()
				select {
				case <-ctx.Done():
					break loop
				case <-doneC: // wait for the service to stop before restarting.
				}

				stoppedAt = &now
				warningTimer.Reset(warningDuration)
			case rpc.CommandSignalRestart:
				cancel()
				stoppedAt = &now
				warningTimer.Reset(warningDuration)
				select {
				case <-ctx.Done():
					break loop
				case <-doneC: // wait for the service to stop before restarting.
				}
				// recreate the service context.
				sctx, cancel = NewServiceContextWithCancel(ctx, ds)
				doneC = m.runService(sctx, ds, updateC)
				stoppedAt = nil

			case rpc.CommandSignalReload:
				// do nothing, the service is already reloading.
				sctx.Log(log.LevelDebug, "service is reloading....")
			}

		case <-doneC:
			doneC = nil // can always read from closed channel, disable it
			// do nothing, we are done.
			sctx.Log(log.LevelWarning, "service has been stopped")
			stoppedAt = &now
			warningTimer.Reset(warningDuration)
		}
	}

	// wait for the service to stop before exiting.
	if doneC != nil {
		<-doneC
	}
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

func (m RunUntilSuccessManager) Manage(ctx context.Context, ds DaemonService, updateC chan<- StateUpdate) {
	sctx, cancel := NewServiceContextWithCancel(ctx, ds)
	defer cancel()

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
