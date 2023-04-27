package rxd

import (
	"context"
	"fmt"
	"sync"
)

type manager struct {
	ctx       context.Context
	cancelCtx context.CancelFunc

	wg *sync.WaitGroup

	services []*ServiceContext

	logC chan LogMessage

	stopCh chan struct{}

	svcCtx    context.Context
	svcCancel context.CancelFunc
}

// informed is a struct used by the manager notifier routine
// to track whether a parent services depedent children have been informed or not.
type informed struct {
	stopC     chan struct{}
	completed bool
	mu        *sync.Mutex
}

func newInformed() *informed {
	return &informed{
		stopC:     make(chan struct{}),
		completed: true,
		mu:        new(sync.Mutex),
	}
}

func (i *informed) reset() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.stopC = make(chan struct{})
	i.completed = true
}

func (i *informed) close() {
	i.mu.Lock()
	defer i.mu.Unlock()
	close(i.stopC)
}

func (i *informed) setComplete(v bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.completed = v
}

func NewManager(services []*ServiceContext) *manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &manager{
		ctx:       ctx,
		cancelCtx: cancel,
		services:  services,
		wg:        new(sync.WaitGroup),
		// stopCh is closed by daemon to signal to manager to stop services
		stopCh: make(chan struct{}),
	}
}

func (m *manager) setLogCh(logC chan LogMessage) {
	m.logC = logC
}

// startService is run by manager in its own routine
// its a service wrapper so lifecycle stages can be controlled
// and relayed behind the scenes.
func (m *manager) startService(serviceCtx *ServiceContext) {
	defer m.wg.Done()

	serviceCtx.setLogChannel(m.logC)
	serviceCtx.setIsStopped(false)

	// All services begin at Init stage
	var svcResp ServiceResponse = NewResponse(nil, InitState)
	service := serviceCtx.service

	for {
		// Every service attempts to notify any services that were set during setup via UsingServiceNotify option.
		serviceCtx.notifyStateChange(svcResp.NextState)
		// Determine the next state the service should be in.
		// Run the method associated with the next state.
		switch svcResp.NextState {

		case InitState:
			svcResp = service.Init(serviceCtx)
			if svcResp.Error != nil {
				m.logC <- NewLog(svcResp.Error.Error(), Error)
			}

		case IdleState:
			svcResp = service.Idle(serviceCtx)
			if svcResp.Error != nil {
				serviceCtx.LogError(svcResp.Error.Error())
			}

		case RunState:
			svcResp = service.Run(serviceCtx)
			if svcResp.Error != nil {
				serviceCtx.LogError(svcResp.Error.Error())
			}

			// Enforce Run policies
			switch serviceCtx.opts.runPolicy {
			case RunOncePolicy:
				// regardless of success/fail, we exit
				svcResp.NextState = ExitState

			case RetryUntilSuccessPolicy:
				if svcResp.Error == nil {
					svcResp := service.Stop(serviceCtx)
					if svcResp.Error != nil {
						continue
					}
					serviceCtx.setIsStopped(true)
					// If Run didnt error, we assume successful run once and stop service.
					svcResp.NextState = ExitState
				}
			}
		case StopState:
			svcResp = service.Stop(serviceCtx)
			if svcResp.Error != nil {
				serviceCtx.LogError(svcResp.Error.Error())
			}
			serviceCtx.setIsStopped(true)
			// Always force Exit after Stop is called.
			svcResp.NextState = ExitState

		case ExitState:
			if !serviceCtx.isStopped {
				serviceCtx.notifyStateChange(StopState)
				// Ensure we still run Stop in case the user sent us ExitState from any other lifecycle method
				svcResp = service.Stop(serviceCtx)
				if svcResp.Error != nil {
					m.logC <- NewLog(svcResp.Error.Error(), Error)
				}
				serviceCtx.setIsStopped(true)
			}
			serviceCtx.notifyStateChange(ExitState)
			serviceCtx.LogDebug("shutting down")
			// if a close signal hasnt been sent to the service.
			serviceCtx.shutdown()
			return
		}
	}
}

// start is run by daemon in its own routine
// manager also handles spinning up each service in its own routine
// as well as a notifier routine per each service that is considered
// a parent with dependent services, any service that AddDependentService()
// was called on.
func (m *manager) start() (exitErr error) {
	defer func() {
		// capture any panics, convert to error to return
		if rErr := recover(); rErr != nil {
			exitErr = fmt.Errorf("%s", rErr)
		}

		close(m.stopCh)
	}()

	go func() {
		// Watch for stop signal, perform shutdown
		m.logC <- NewLog("manager watching for stop signal....", Debug)
		<-m.stopCh
		m.logC <- NewLog("manager received stop signal", Debug)
		m.shutdown()
		// signal complete using context
		m.cancelCtx()
	}()

	for _, service := range m.services {
		m.wg.Add(1)
		if len(service.dependents) > 0 {
			// start a notifier watcher routine only for services that have children to notify of state change.
			go m.notifier(service)
		}
		// Start each service in its own routine logic / conditional lifecycle.
		go m.startService(service)
	}

	m.logC <- NewLog("Started all services...", Info)

	// Main thread blocking forever infinite loop to select between
	//  listening for OS Signal and/or errors to print from each service.
	m.wg.Wait()
	m.logC <- NewLog("All services have stopped running", Info)
	return exitErr
}

// shutdown handles iterating over any remaining services that still may be running
// calling each services shutdown method if it hasnt already been called by the service itself
// on a manual Stop/Exit state before manager began its own shutdown.
func (m *manager) shutdown() {
	var totalRunning int
	// sends a signal to each service to inform them to stop running.
	for _, serviceCtx := range m.services {
		if !serviceCtx.isShutdown {
			m.logC <- NewLog(fmt.Sprintf("Signaling stop of service: %s", serviceCtx.name), Debug)
			serviceCtx.shutdown()
			totalRunning++
		}
	}

	if totalRunning > 0 {
		m.logC <- NewLog(fmt.Sprintf("%d remaining services signaled to shut down.", totalRunning), Debug)
	}
}

// notifier is a goroutine launched only per parent with dependent services
// it holds some last known state of parent and state of which dependent
// services have been informed or have yet to be informed which is uses
// its own routines to actively attempt relaying that state across a channel.
// Since that channel blocks until the dependent receives it, if the parent state changes
// between that time, to prevent out-of-sync issue we kill the go routine immediately on
// state change and launch a new one.
func (m *manager) notifier(parent *ServiceContext) {
	lastState := InitState
	informedChildren := make(map[*ServiceContext]*informed)

	for {
		select {
		case <-m.ctx.Done():
			return
		case state := <-parent.stateC:
			lastState = state
			// always watch for parent state change
			for childSvc, interestedStates := range parent.dependents {

				if _, ok := interestedStates[state]; !ok {
					// if its not a state we care about, skip it.
					continue
				}

				informedChild, exists := informedChildren[childSvc]
				if !exists {
					informedChild = newInformed()
					informedChildren[childSvc] = informedChild
				}

				if !informedChild.completed {
					// if we are doing a state change and my previous routine is still hanging.
					// kill it and send the newer state change.
					informedChild.close()
				}
				// it has completed, so send the next update.
				informedChild.reset()

				// Go routine that always attempts to send the last known state.
				go func(svc *ServiceContext, ls State, i *informed) {
					defer func() {
						i.close()
						i.setComplete(true)
					}()

					if svc.isShutdown {
						// if the svc is already shutdown dont bother.
						return
					}
					select {
					case <-m.ctx.Done():
						return
					case <-i.stopC:
						// so we can kill this routine if we are stuck hanging and another update came in while no one was listening.
						return
					case svc.stateChangeC <- state:
						// hold open forever until childSvc is ready to receive.
						i.setComplete(true)
						return
					}
				}(childSvc, lastState, informedChild)
			}
		}
	}

}
