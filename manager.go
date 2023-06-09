package rxd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ambitiousfew/intracom"
)

type manager struct {
	ctx       context.Context
	cancelCtx context.CancelFunc

	wg *sync.WaitGroup

	services []*ServiceContext

	// states map reflects the state of a given service
	states       States
	stateUpdateC chan StateUpdate
	intracom     *intracom.Intracom[[]byte]

	log Logging
	// used to signal that manager has exited start() therefore is trying to stop which triggers shutdown()
	stopCh chan struct{}

	shutdownCalled atomic.Int32

	mu *sync.Mutex
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
		completed: false,
		mu:        new(sync.Mutex),
	}
}

func (i *informed) reset() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.stopC = make(chan struct{})
	i.completed = false
}

func (i *informed) close() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.completed {
		close(i.stopC)
	}
	i.completed = true
}

func newManager(services []*ServiceContext) *manager {
	ctx, cancel := context.WithCancel(context.Background())
	ic := intracom.New[[]byte]()

	return &manager{
		ctx:       ctx,
		cancelCtx: cancel,
		services:  services,
		// N services * 2 (possible states)
		stateUpdateC: make(chan StateUpdate, len(services)*2),
		wg:           new(sync.WaitGroup),
		// stopCh is closed by daemon to signal to manager to stop services
		stopCh: make(chan struct{}),
		// intercom will be passed to each service to share for inter-service comms
		intracom: ic,
		mu:       new(sync.Mutex),
	}
}

// setLogger sets the passed in logging instance as the logger for manager and all services within manager.
func (m *manager) setLogger(logger Logging) {
	m.log = logger
}

// startService is run by manager in its own routine
// its a service wrapper so lifecycle stages can be controlled
// and relayed behind the scenes.
func (m *manager) startService(serviceCtx *ServiceContext) {
	defer m.wg.Done()

	// All services begin at Init stage
	var svcResp ServiceResponse = NewResponse(nil, InitState)
	service := serviceCtx.service

	for {
		m.updateServiceState(serviceCtx.Name, svcResp.NextState)
		// Every service attempts to notify any services that were set during setup via UsingServiceNotify option.
		serviceCtx.notifyStateChange(svcResp.NextState)
		// Determine the next state the service should be in.
		// Run the method associated with the next state.
		switch svcResp.NextState {

		case InitState:

			svcResp = service.Init(serviceCtx) //blocking
			if svcResp.Error != nil {
				serviceCtx.Log.Errorf("%s %s", serviceCtx.Name, svcResp.Error.Error())
			}

		case IdleState:
			svcResp = service.Idle(serviceCtx)
			if svcResp.Error != nil {
				serviceCtx.Log.Errorf("%s %s", serviceCtx.Name, svcResp.Error.Error())
			}

		case RunState:
			svcResp = service.Run(serviceCtx)
			if svcResp.Error != nil {
				serviceCtx.Log.Errorf("%s %s", serviceCtx.Name, svcResp.Error.Error())
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
					serviceCtx.stopCalled.Store(1)
					// If Run didnt error, we assume successful run once and stop service.
					svcResp.NextState = ExitState
				}
			}
		case StopState:
			svcResp = service.Stop(serviceCtx)
			if svcResp.Error != nil {
				serviceCtx.Log.Errorf("%s %s", serviceCtx.Name, svcResp.Error.Error())
			}
			serviceCtx.stopCalled.Store(1)
			// Always force Exit after Stop is called.
			svcResp.NextState = ExitState

		case ExitState:
			if !serviceCtx.hasStopped() {
				serviceCtx.notifyStateChange(StopState)
				// Ensure we still run Stop in case the user sent us ExitState from any other lifecycle method
				svcResp = service.Stop(serviceCtx)
				if svcResp.Error != nil {
					serviceCtx.Log.Errorf("%s %s", serviceCtx.Name, svcResp.Error.Error())
				}
				serviceCtx.stopCalled.Store(1)
			}
			// if a close signal hasnt been sent to the service.
			serviceCtx.shutdown()

			close(serviceCtx.stateC)
			return
		}
	}
}

func (m *manager) preStartCheck() error {
	serviceNames := make(map[string]struct{})
	for _, service := range m.services {
		if _, exists := serviceNames[service.Name]; exists {
			return fmt.Errorf("manager error: the service named '%s' overlaps with another service name", service.Name)
		}
		serviceNames[service.Name] = struct{}{}
	}
	return nil
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

		m.log.Debug("manager closing intracom")
		m.intracom.Close()
		m.log.Debug("manager has finished close on intracom")
		close(m.stopCh) // signal to daemon signal watcher
	}()

	var parents int
	var dependents int
	var independents int
	var total int

	for _, service := range m.services {
		s := service
		if len(service.dependents) > 0 {
			// start a notifier watcher routine only for services that have children to notify of state change.
			parents++
			dependents += len(service.dependents)
			go m.notifier(service)
		} else {
			independents++
		}
		s.setIntracom(m.intracom)
		// Start each service in its own routine logic / conditional lifecycle.
		m.wg.Add(1)
		go m.startService(s)
	}

	total = parents + dependents + independents

	if parents == 0 && dependents == 0 {
		m.log.Debugf("manager has started %d services", total)
	} else {
		m.log.Debugf("manager has started %d services: %d (p), %d (d), and %d (i)", total, parents, dependents, independents)
	}

	// Main thread blocking forever infinite loop to select between
	//  listening for OS Signal and/or errors to print from each service.
	m.wg.Wait()

	// all services are done running their lifecycles beyond this point.
	m.log.Debug("manager has stopped running all services")
	return exitErr
}

func (m *manager) hasShutdown() bool {
	return m.shutdownCalled.Load() == 1
}

// shutdown handles iterating over any remaining services that still may be running
// calling each services shutdown method if it hasnt already been called by the service itself
// on a manual Stop/Exit state before manager began its own shutdown.
func (m *manager) shutdown() {
	if m.shutdownCalled.Swap(1) == 1 {
		// if the old val is already 1, then shutdown has already been called before, dont run twice.
		m.log.Debug("manager shutdown was called twice....")
		return
	}
	var wg sync.WaitGroup

	var totalRunning int
	for _, serviceCtx := range m.services {
		if !serviceCtx.hasShutdown() && !serviceCtx.isDependent {
			wg.Add(1)
			// When shutting down only look for services who are not added as a dependent.
			// This lets us signal shutdown to any parent service or individual service.
			// Parent services will signal shutdown to all their child dependents.
			m.log.Debugf("manager is signaling stop of service: %s", serviceCtx.Name)
			svc := serviceCtx // rebind loop variable
			// Signal all non-dependent services to shutdown without hanging on for the previous shutdown call.
			go func() {
				defer wg.Done()
				svc.shutdown()
			}()

			totalRunning++
		}
	}

	if totalRunning > 0 {
		m.log.Debugf("manager has signaled %d services to shut down.", totalRunning)
	}
	// wait for all shutdown routine calls to finish.
	m.log.Debug("manager is waiting for all services to shutdown")
	wg.Wait()
	m.log.Debug("manager cancelling context")
	m.cancelCtx()
}

func (m *manager) updateServiceState(serviceName string, state State) {
	m.stateUpdateC <- StateUpdate{Name: serviceName, State: state}
}

func (m *manager) serviceStateWatcher(stopC chan struct{}) {
	states := make(map[string]State)

	for _, sc := range m.services {
		states[sc.Name] = InitState
	}

	for {
		select {
		case <-stopC:
			return
		case update := <-m.stateUpdateC:
			currState, found := states[update.Name]
			if found && currState == update.State {
				// skip any non-changes from previous state.
				continue
			}

			states[update.Name] = update.State

			statesBytes, err := json.Marshal(states)
			if err != nil {
				m.log.Debugf("manager error marshalling updated states map: %s", err)
				continue
			}

			m.intracom.Publish(internalServiceStates, statesBytes)
		}
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

	informedChildren := make(map[*ServiceContext]*informed)

	for {
		select {
		case <-m.ctx.Done():
			return
		case state := <-parent.stateC:
			// Every service announces its own state change.
			// notifier listens to any service considered a parent for these changes.
			if state == "" {
				// if we receive a close(), a nil would be sent which becomes nil of State which is ""
				return
			}

			// Store the current state as our last known state.
			lastState := state

			for childSvc, interestedStates := range parent.dependents {
				// Figure out if the dependent children care about the current state change.
				if _, ok := interestedStates[lastState]; !ok {
					// if its not a state we care about, skip it.
					continue
				}

				if childSvc.hasShutdown() {
					// if the child service is already shutdown, skip...
					continue
				}

				informedChild, exists := informedChildren[childSvc]
				if !exists {
					informedChild = newInformed()
					informedChildren[childSvc] = informedChild
				} else {
					// informedChild has already been created before, its still running using outdated state
					// and its likely still hanging waiting to send that state. We want to signal it to stop
					// trying so we can send the newer (current) state transition being notified for now.
					// state change.
					informedChild.close()
					// reset channel/completed so send the next update.
					informedChild.reset()
				}

				// Go routine that always attempts to send the last known state of parent to dependent service.
				go func(svc *ServiceContext, ls State, i *informed) {
					defer func() {
						// i.setComplete(true)
						i.close()
					}()

					if svc.hasShutdown() {
						// if the svc is already shutdown dont bother.
						return
					}

					// TODO: We could possible warn about a child service taking too long to receive the parents state change.
					select {
					case <-m.ctx.Done():
						return
					case <-i.stopC:
						// so we can kill this routine if we are stuck hanging and another update came in while no one was listening.
						return
					case svc.stateChangeC <- ls:
						// hold open forever until childSvc is ready to receive.
						// i.setComplete(true)
						return
					}
				}(childSvc, lastState, informedChild)
			}
		}
	}

}
