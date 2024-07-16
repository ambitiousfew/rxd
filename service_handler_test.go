package rxd

import (
	"fmt"

	"github.com/ambitiousfew/rxd/log"
)

type mockServiceHandler struct{}

func (h mockServiceHandler) Handle(sctx ServiceContext, ds DaemonService, updateState func(string, State)) {
	defer func() {
		// if any panics occur with the users defined service runner, recover and push error out to daemon logger.
		if r := recover(); r != nil {
			sctx.Log(log.LevelError, fmt.Sprintf("recovered from a panic: %v", r))
		}
	}()

	var state State = StateInit
	var hasStopped bool

	for state != StateExit {
		var err error
		// signal the current state we are about to enter. to the daemon states watcher.
		updateState(ds.Name, state)

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
				state, err = ds.Runner.Init(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = StateInit
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
	updateState(ds.Name, StateExit)
}
