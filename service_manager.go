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
	Manage(parent context.Context, dstate DaemonState, mService ManagedService)
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
}

func NewDefaultManager(opts ...ManagerOption) RunContinuousManager {
	timeouts := make(ManagerStateTimeouts)
	m := RunContinuousManager{
		DefaultDelay:  100 * time.Millisecond,
		StartupDelay:  100 * time.Millisecond,
		StateTimeouts: timeouts,
		LogWarning:    false,
		WarnDuration:  10 * time.Second,
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
			return StateRun, nil
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

func (m RunContinuousManager) runService(sctx ServiceContext, ds DaemonState, ms ManagedService) <-chan struct{} {
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

				switch state {
				case StateInit:
					if initializer, ok := ms.Runner.(ServiceInitializer); ok {
						ds.NotifyState(ms.Name, state)
						if err := initializer.Init(sctx); err != nil {
							sctx.Log(log.LevelError, err.Error())
							state = StateStop
							continue
						}
					}
					state = StateIdle

				case StateIdle:
					if idler, ok := ms.Runner.(ServiceIdler); ok {
						ds.NotifyState(ms.Name, state)
						if err := idler.Idle(sctx); err != nil {
							sctx.Log(log.LevelError, err.Error())
							state = StateStop
							continue
						}
					}
					state = StateRun
				case StateRun:
					ds.NotifyState(ms.Name, state)
					if err := ms.Runner.Run(sctx); err != nil {
						sctx.Log(log.LevelError, err.Error())
					}
					state = StateStop
				case StateStop:
					if stopper, ok := ms.Runner.(ServiceStopper); ok {
						ds.NotifyState(ms.Name, state)
						if err := stopper.Stop(sctx); err != nil {
							sctx.Log(log.LevelError, err.Error())
						}
						hasStopped = true
					}

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
			if stopper, ok := ms.Runner.(ServiceStopper); ok {

				timedCtx, timedCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer timedCancel()

				stopCtx := sctx.WithParent(timedCtx)

				sctx, cancel := NewServiceContextWithCancel(stopCtx, ms.Name, ds)
				defer cancel()

				ds.NotifyState(ms.Name, StateStop)
				if err := stopper.Stop(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
			}
		}

		ds.NotifyState(ms.Name, StateExit)

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
func (m RunContinuousManager) Manage(ctx context.Context, ds DaemonState, ms ManagedService) {
	logger := ds.Logger("manager", ms.Name)

	logger.Log(log.LevelDebug, "service manager started")

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

	configC := ds.LoadSignal()

	var lastUpdate int64
	doneC := m.runService(sctx, ds, ms)

loop:
	for {
		now := time.Now()
		select {
		case <-ctx.Done():
			cancel()
			break loop
		case ts, open := <-configC:
			// config reload broadcast received
			if !open {
				break loop
			}
			lastUpdate = ts

			if lastUpdate == 0 {
				// skip if the ts is 0 (intracom subscriber initial message)
				continue
			}

			logger.Log(log.LevelDebug, "config reload update received", log.Int("timestamp", ts))
			if ts < lastUpdate {
				// skip if the config is older than the last update.
				continue
			}
			lastUpdate = ts

			cancel()
			select {
			case <-ctx.Done():
				break loop
			case <-doneC:
				// wait for service to exit before reloading the config.
			}

			fields := ds.loader.Load(ctx)

			sctx, cancel = NewServiceContextWithCancel(ctx, ms.Name, ds)
			if reloader, ok := ms.Runner.(ServiceLoader); ok {
				if err := reloader.Load(sctx, fields); err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
			}

			// restart the service
			doneC = m.runService(sctx, ds, ms)
		case <-warningTimer.C:
			if stoppedAt == nil {
				// stopped wasnt set
				continue
			}

			timeDiff := time.Since(*stoppedAt)
			logger.Log(log.LevelWarning, "service has been stopped for over a minute", log.String("duration", friendlyTimeDiff(timeDiff)))
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
				doneC = m.runService(sctx, ds, ms)
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
				doneC = m.runService(sctx, ds, ms)
				stoppedAt = nil

			case rpc.CommandSignalReload:
				// do nothing, the service is already reloading.
				logger.Log(log.LevelDebug, "service is reloading....")
			}

		case <-doneC:
			doneC = nil // can always read from closed channel, disable it
			// do nothing, we are done.
			logger.Log(log.LevelWarning, "service has been stopped")
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

func (m RunUntilSuccessManager) Manage(ctx context.Context, ds DaemonState, ms ManagedService) {
	sctx, cancel := NewServiceContextWithCancel(ctx, ms.Name, ds)
	defer cancel()

	defer func() {
		// if any panics occur with the users defined service runner, recover and push error out to daemon logger.
		if r := recover(); r != nil {
			sctx.Log(log.LevelError, fmt.Sprintf("recovered from a panic: %v", r))
		}
	}()

	// if the service is reloadable

	ticker := time.NewTicker(m.StartupDelay)
	defer ticker.Stop()

	var hasStopped bool
	// run continous manager will always start from the init state.
	var state State = StateInit
	select {
	case <-sctx.Done():
		state = StateExit
	case <-ticker.C:
		ticker.Reset(m.DefaultDelay)
	}

	for state != StateExit {
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
				if initializer, ok := ms.Runner.(ServiceInitializer); ok {
					ds.NotifyState(ms.Name, state)
					if err := initializer.Init(sctx); err != nil {
						sctx.Log(log.LevelError, err.Error())
						state = StateStop
						continue
					}
				}
				state = StateIdle

			case StateIdle:
				if idler, ok := ms.Runner.(ServiceIdler); ok {
					ds.NotifyState(ms.Name, state)
					if err := idler.Idle(sctx); err != nil {
						sctx.Log(log.LevelError, err.Error())
						state = StateStop
						continue
					}
				}
				state = StateRun
			case StateRun:
				ds.NotifyState(ms.Name, state)
				if err := ms.Runner.Run(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
					continue
				}
				// run exited successfully, we can exit the loop.
				state = StateExit
			case StateStop:
				if stopper, ok := ms.Runner.(ServiceStopper); ok {
					ds.NotifyState(ms.Name, state)
					if err := stopper.Stop(sctx); err != nil {
						sctx.Log(log.LevelError, err.Error())
					}
					hasStopped = true
				}
				state = StateInit
			}
		}

	}

	if !hasStopped {
		if stopper, ok := ms.Runner.(ServiceStopper); ok {
			ds.NotifyState(ms.Name, StateStop)
			// ensure that if any lifecycle ran after stop, we run stop again (for cleanup).
			if err := stopper.Stop(sctx); err != nil {
				sctx.Log(log.LevelError, err.Error())
			}
		}
	}

	// push final state to the daemon states watcher.
	ds.NotifyState(ms.Name, StateExit)

}
