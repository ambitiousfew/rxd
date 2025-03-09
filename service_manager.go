package rxd

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ambitiousfew/rxd/log"
	"github.com/ambitiousfew/rxd/pkg/rpc"
)

type ConfigPolicy uint8

const (
	// ConfigPolicyDoNothing will not call the config loader function and will not stop the service.
	ConfigPolicyDoNothing ConfigPolicy = iota
	// ConfigPolicyStopFirst will stop the service before calling the config loader function.
	ConfigPolicyStopFirst
	// ConfigPolicyDontStop will call the config loader function without stopping the service.
	ConfigPolicyDontStop
)

// ServiceManager interface defines the methods that a service handler must implement
type ServiceManager interface {
	Manage(parent context.Context, dstate DaemonState, mService ManagedService, updateC chan<- StateUpdate)
	LoadPolicy() ConfigPolicy
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
	LogWarning    bool          // if true, log a warning if the service has been stopped for longer than WarnDuration.
	WarnDuration  time.Duration // if the service has been stopped for longer than this duration, log a warning.
	ConfigPolicy  ConfigPolicy  // ConfigPolicy defines the policy for the config loader.
}

func NewDefaultManager(opts ...ManagerOption) RunContinuousManager {
	timeouts := make(ManagerStateTimeouts)
	m := RunContinuousManager{
		DefaultDelay:  100 * time.Millisecond,
		StartupDelay:  100 * time.Millisecond,
		StateTimeouts: timeouts,
		LogWarning:    false,
		WarnDuration:  10 * time.Second,
		// noop config loader
		ConfigPolicy: ConfigPolicyStopFirst,
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

func (m RunContinuousManager) LoadPolicy() ConfigPolicy {
	return ConfigPolicyStopFirst
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

func (m RunContinuousManager) runService(sctx ServiceContext, ds DaemonState, ms ManagedService, updateC chan<- StateUpdate) <-chan struct{} {
	doneC := make(chan struct{})

	go func(sctx ServiceContext, ms ManagedService) {
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
				updateC <- StateUpdate{Name: ms.Name, State: state}

				switch state {
				case StateInit:
					if err := ms.Runner.Init(sctx); err != nil {
						sctx.Log(log.LevelError, err.Error())
						state = StateStop
						continue
					}
					state = StateIdle
				case StateIdle:
					if err := ms.Runner.Idle(sctx); err != nil {
						sctx.Log(log.LevelError, err.Error())
						state = StateStop
						continue
					}
					state = StateRun
				case StateRun:
					if err := ms.Runner.Run(sctx); err != nil {
						sctx.Log(log.LevelError, err.Error())
					}
					state = StateStop
				case StateStop:
					if err := ms.Runner.Stop(sctx); err != nil {
						sctx.Log(log.LevelError, err.Error())
					}
					hasStopped = true

					state = StateInit
				default:
					state = StateExit
					// log
					sctx.Log(log.LevelError, "unknown state, exiting service")
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
			timedCtx, timedCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer timedCancel()

			stopCtx := sctx.WithParent(timedCtx)

			sctx, cancel := NewServiceContextWithCancel(stopCtx, ms.Name, ds)
			defer cancel()

			if err := ms.Runner.Stop(sctx); err != nil {
				sctx.Log(log.LevelError, err.Error())
			}
		}

		updateC <- StateUpdate{Name: ms.Name, State: StateExit}

	}(sctx, ms)

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
func (m RunContinuousManager) Manage(ctx context.Context, ds DaemonState, ms ManagedService, updateC chan<- StateUpdate) {
	warningTimer := time.NewTimer(m.WarnDuration)
	warningTimer.Stop()

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
			case cmd := <-ms.CommandC:
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

	var stoppedAt *time.Time

	sctx, cancel := NewServiceContextWithCancel(ctx, ms.Name, ds)
	defer cancel()

	var lastUpdate int64
	doneC := m.runService(sctx, ds, ms, updateC)

loop:
	for {
		now := time.Now()
		select {
		case <-ctx.Done():
			cancel()
			break loop
		case ts, open := <-ds.configC:
			if !open {
				break loop
			}
			lastUpdate = ts

			if lastUpdate == 0 {
				// skip if the ts is 0 (intracom subscriber initial message)
				continue
			}

			sctx.Log(log.LevelDebug, "config update received", log.Int("timestamp", ts))

			if ts < lastUpdate {
				// skip if the config is older than the last update.
				continue
			}
			lastUpdate = ts

			switch m.ConfigPolicy {
			// do nothing
			case ConfigPolicyStopFirst:
				cancel()
				select {
				case <-ctx.Done():
					break loop
				case <-doneC:
					// wait for service to exit before reloading the config.
				}

			default:
				continue // skip, do nothing if any other option.
			}

			// reload the config now
			err := ds.configLoader.Load(ctx, ms.ServiceLoaderFn)
			if err != nil {
				sctx.Log(log.LevelError, err.Error())
			}

			switch m.ConfigPolicy {
			case ConfigPolicyStopFirst:
				// start the stopped service back up.
				sctx, cancel = NewServiceContextWithCancel(ctx, ms.Name, ds)
				doneC = m.runService(sctx, ds, ms, updateC)
			default:
				// do nothing for the others.
			}

		case <-warningTimer.C:
			if stoppedAt == nil {
				// stopped wasnt set
				continue
			}

			timeDiff := time.Since(*stoppedAt)
			sctx.Log(log.LevelWarning, "service has been stopped for over a minute", log.String("duration", friendlyTimeDiff(timeDiff)))
			if m.LogWarning {
				warningTimer.Reset(m.WarnDuration)
			}
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
				sctx, cancel = NewServiceContextWithCancel(ctx, ms.Name, ds)
				doneC = m.runService(sctx, ds, ms, updateC)
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
				if m.LogWarning {
					warningTimer.Reset(m.WarnDuration)
				}
			case rpc.CommandSignalRestart:
				cancel()
				stoppedAt = &now
				if m.LogWarning {
					warningTimer.Reset(m.WarnDuration)
				}
				select {
				case <-ctx.Done():
					break loop
				case <-doneC: // wait for the service to stop before restarting.
				}
				// recreate the service context.
				sctx, cancel = NewServiceContextWithCancel(ctx, ms.Name, ds)
				doneC = m.runService(sctx, ds, ms, updateC)
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
			if m.LogWarning {
				warningTimer.Reset(m.WarnDuration)
			}

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

func (m RunUntilSuccessManager) LoadPolicy() ConfigPolicy {
	return ConfigPolicyDoNothing
}

func (m RunUntilSuccessManager) Manage(ctx context.Context, ds DaemonState, ms ManagedService, updateC chan<- StateUpdate) {
	sctx, cancel := NewServiceContextWithCancel(ctx, ms.Name, ds)
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
		if err := ms.Runner.Init(sctx); err != nil {
			sctx.Log(log.LevelError, err.Error())
			state = StateStop
		}
		state = StateIdle
		ticker.Reset(m.DefaultDelay)
	}

	for state != StateExit {
		// relay the current state we are about to enter to the daemon's states watcher.
		updateC <- StateUpdate{Name: ms.Name, State: state}

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
				if err := ms.Runner.Init(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
					continue
				}
				state = StateIdle

			case StateIdle:
				if err := ms.Runner.Idle(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
					continue
				}
				state = StateRun

			case StateRun:
				if err := ms.Runner.Run(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
					continue
				}
				// run exited successfully, we can exit the loop.
				state = StateExit
			case StateStop:
				if err := ms.Runner.Stop(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
				state = StateInit
				hasStopped = true
			}
		}

	}

	if !hasStopped {
		// ensure that if any lifecycle ran after stop, we run stop again (for cleanup).
		if err := ms.Runner.Stop(sctx); err != nil {
			sctx.Log(log.LevelError, err.Error())
		}
	}

	// push final state to the daemon states watcher.
	updateC <- StateUpdate{Name: ms.Name, State: StateExit}

}
