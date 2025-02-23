package rxd

import (
	"context"
	"fmt"

	"github.com/ambitiousfew/rxd/log"
)

type mockServiceManager struct{}

func (h mockServiceManager) Manage(ctx context.Context, ds DaemonService, updateC chan<- StateUpdate) {
	// func (h mockServiceManager) Manage(sctx ServiceContext, ds DaemonService, updateC chan<- StateUpdate) {

	sctx, cancel := NewServiceContextWithCancel(ctx, ds)
	defer cancel()

	defer func() {
		// if any panics occur with the users defined service runner, recover and push error out to daemon logger.
		if r := recover(); r != nil {
			sctx.Log(log.LevelError, fmt.Sprintf("recovered from a panic: %v", r))
		}
	}()

	var state State = StateInit
	var hasStopped bool

	for state != StateExit {
		// signal the current state we are about to enter. to the daemon states watcher.
		updateC <- StateUpdate{Name: ds.Name, State: state}

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
				err := ds.Runner.Init(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateInit
				}
				// reset the hasStopped flag since we just restarted the service.
				hasStopped = false

			case StateIdle:
				err := ds.Runner.Idle(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateStop
				}
			case StateRun:
				err := ds.Runner.Run(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
				}
			case StateStop:
				err := ds.Runner.Stop(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
				}

				// flip hasStopped to true to ensure we don't run stop again if Exit is next.
				hasStopped = true
				continue // skip the default timeout reset
			}

		}
	}

	if !hasStopped {
		// we are only wanting to ensure stop is run if it hasn't been run already then exit.
		err := ds.Runner.Stop(sctx)
		if err != nil {
			sctx.Log(log.LevelError, err.Error())
		}
	}
	// push final state to the daemon states watcher.
	updateC <- StateUpdate{Name: ds.Name, State: StateExit}
}
