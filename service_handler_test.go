package rxd

import (
	"context"

	"github.com/ambitiousfew/rxd/log"
)

type mockServiceManager struct{}

func (h mockServiceManager) Manage(ctx context.Context, ds DaemonState, ms ManagedService) {
	sctx, cancel := NewServiceContextWithCancel(ctx, ms.Name, ds)
	defer cancel()

	var state State = StateInit
	var hasStopped bool

	for state != StateExit {
		if hasStopped {
			// not entering Exit after stop was true, reset hasStopped
			hasStopped = false
		}

		select {
		case <-sctx.Done():
			// push to exit.
			state = StateExit
		default:
			switch state {
			case StateInit:
				if initializer, ok := ms.Runner.(ServiceInitializer); ok {
					// inform daemon states watcher of the state change.
					ds.NotifyState(ms.Name, state)
					err := initializer.Init(sctx)
					if err != nil {
						sctx.Log(log.LevelError, err.Error())
						state = StateInit
					}
				}

			case StateIdle:
				if idler, ok := ms.Runner.(ServiceIdler); ok {
					// inform daemon states watcher of the state change.
					ds.NotifyState(ms.Name, state)
					err := idler.Idle(sctx)
					if err != nil {
						sctx.Log(log.LevelError, err.Error())
						state = StateStop
					}
				}
			case StateRun:
				// inform daemon states watcher of the state change.
				ds.NotifyState(ms.Name, state)

				err := ms.Runner.Run(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
				}

			case StateStop:
				if stopper, ok := ms.Runner.(ServiceStopper); ok {
					// inform daemon states watcher of the state change.
					ds.NotifyState(ms.Name, state)

					err := stopper.Stop(sctx)
					if err != nil {
						sctx.Log(log.LevelError, err.Error())
					}
					// flip hasStopped to true to ensure we don't run stop again if Exit is next.
					hasStopped = true
				}
			}

		}
	}

	if !hasStopped {
		if stopper, ok := ms.Runner.(ServiceStopper); ok {
			// we are only wanting to ensure stop is run if it hasn't been run already then exit.
			err := stopper.Stop(sctx)
			if err != nil {
				sctx.Log(log.LevelError, err.Error())
			}
		}
	}
	// push final state to the daemon states watcher.
	ds.NotifyState(ms.Name, StateExit)
}
