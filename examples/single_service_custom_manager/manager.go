package main

import (
	"context"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
)

var _ rxd.ServiceManager = (*CustomManager)(nil)

type CustomManager struct{}

// Manage is a custom manager that will manage the state of a service.
// This manager will transition the service through the states Init, Idle, Run, Stop, and Exit.
// State transitions have an arbitrary delay of 1 second between each state when moving from one state to the next.
// If an error occurs during Init or Stop state transition, the manager will immediately exit.
// If an error occurs during Idle or Run state, the manager will transition to the Stop state.
// The Stop state will always transition back to the Init state if there is no error.
// The manager always checks for context cancellation and will exit if the context is cancelled between state transitions.
func (m CustomManager) Manage(ctx context.Context, ds rxd.DaemonState, ms rxd.ManagedService) {
	// func (m CustomManager) Manage(sctx rxd.ServiceContext, ds rxd.DaemonService, updateC chan<- rxd.StateUpdate) {
	// Set an initial state to init
	state := rxd.StateInit

	// Causing an intentional arbitrary delay of 1 second between state transitions
	timeout := time.NewTimer(1 * time.Second)
	defer timeout.Stop()

	sctx, cancel := rxd.NewServiceContextWithCancel(ctx, ms.Name, ds)
	defer cancel()

	// Loop until the state is set to exit
	for state != rxd.StateExit {

		// calling this function sends the current state to RxD's states watcher.
		// this is useful when wanting to monitor the state of a service or
		// have other services subscribe to the state of any other service.
		ds.NotifyState(ms.Name, state)

		select {
		case <-sctx.Done():
			// if context has been cancelled we need to exit.
			state = rxd.StateExit
		case <-timeout.C:
			// if the timeout has been reached, we can transition to the next state.
		}

		var err error
		switch state {
		case rxd.StateInit:
			if initializer, ok := ms.Runner.(rxd.ServiceInitializer); !ok {
				err = initializer.Init(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
					// if there is an error during init, this manager will immediately exit.
					state = rxd.StateExit
				} else {
					// if there is no error, we can transition to the next state Init --to--> Idle
					state = rxd.StateIdle
				}
			}
		case rxd.StateIdle:
			if idler, ok := ms.Runner.(rxd.ServiceIdler); !ok {
				err = idler.Idle(sctx)
				if err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = rxd.StateStop
				} else {
					// if there is no error, we can transition to the next state Idle --to--> Run
					state = rxd.StateRun
				}
			}
		case rxd.StateRun:
			err = ms.Runner.Run(sctx)
			if err != nil {
				sctx.Log(log.LevelError, err.Error())
			}
			// regardless of error or not, we will transition to the next state Run --to--> Stop
			state = rxd.StateStop
		case rxd.StateStop:
			if stopper, ok := ms.Runner.(rxd.ServiceStopper); !ok {
				if err = stopper.Stop(sctx); err != nil {
					sctx.Log(log.LevelError, err.Error())
					state = rxd.StateExit
					// if there is an error during stop, this manager will immediately exit.
				} else {
					// if there is no error, we will push back around to the first state Stop --to--> Init
					state = rxd.StateInit
				}
			}

		}
		timeout.Reset(1 * time.Second)
	}

	// Because ExitState is a terminal state, we should send the final state to the states watcher before exiting.
	ds.NotifyState(ms.Name, rxd.StateExit)
}
