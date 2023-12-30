package rxd

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ambitiousfew/intracom"
	"golang.org/x/exp/slog"
)

type manager struct {
	ctx       context.Context
	cancelCtx context.CancelFunc

	services []*ServiceContext

	stateUpdateC chan StateUpdate

	// intracom *intracom.Intracom[[]byte]
	iSignals *intracom.Intracom[rxdSignal]
	iStates  *intracom.Intracom[States]

	log *slog.Logger
	mu  *sync.Mutex

	shutdownCalled atomic.Int32
}

func newManager(services []*ServiceContext) *manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &manager{
		ctx:       ctx,
		cancelCtx: cancel,
		services:  services,

		// give the state update channel a buffer size of 2x the number of services
		stateUpdateC: make(chan StateUpdate, len(services)*2),
		mu:           new(sync.Mutex),
	}
}

// startService is run by manager in its own routine
// it wraps the service's lifecycle methods in a loop and runs them until the service returns ExitState
func (m *manager) startService(wg *sync.WaitGroup, serviceCtx *ServiceContext) {
	defer wg.Done()

	// All services begin at Init stage
	var svcResp ServiceResponse = NewResponse(nil, InitState)
	service := serviceCtx.service

	for {
		// inform state watcher of this service's upcoming state transition
		m.stateUpdateC <- StateUpdate{Name: serviceCtx.Name, State: svcResp.NextState}

		// Determine the next state the service should be in.
		// Run the method associated with the next state.
		switch svcResp.NextState {

		case InitState:

			svcResp = service.Init(serviceCtx)
			if svcResp.Error != nil {
				serviceCtx.Log.Error(fmt.Sprintf("%s %s", serviceCtx.Name, svcResp.Error.Error()))
			}

		case IdleState:
			svcResp = service.Idle(serviceCtx)
			if svcResp.Error != nil {
				serviceCtx.Log.Error(fmt.Sprintf("%s %s", serviceCtx.Name, svcResp.Error.Error()))
			}

		case RunState:
			svcResp = service.Run(serviceCtx)
			if svcResp.Error != nil {
				serviceCtx.Log.Error(fmt.Sprintf("%s %s", serviceCtx.Name, svcResp.Error.Error()))
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
				serviceCtx.Log.Error(fmt.Sprintf("%s %s", serviceCtx.Name, svcResp.Error.Error()))
			}
			serviceCtx.stopCalled.Store(1)

		case ExitState:
			if !serviceCtx.hasStopped() {
				// inform state watcher of this service's upcoming state transition since we wont loop again after this.
				m.stateUpdateC <- StateUpdate{Name: serviceCtx.Name, State: svcResp.NextState}
				// Ensure we still run Stop in case the user sent us ExitState from any other lifecycle method
				svcResp = service.Stop(serviceCtx)
				if svcResp.Error != nil {
					serviceCtx.Log.Error(fmt.Sprintf("%s %s", serviceCtx.Name, svcResp.Error.Error()))
				}
				serviceCtx.stopCalled.Store(1)
			}

			serviceCtx.shutdown()
			// we are done with this service, exit the service wrapper routine.
			serviceCtx.Log.Debug("service exiting", "service", serviceCtx.Name)
			close(serviceCtx.doneC)
			return

		default:
			serviceCtx.Log.Error(fmt.Sprintf("unknown state '%s' returned from service", svcResp.NextState))
		}

	}
}

// preStartCheck is run before manager starts its own routine
// it ensures that all services have unique names since overlapping
// names would cause issues with services interested in specific service states.
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
// it starts all services and blocks until all services have exited their lifecycles.
// once all services have exited their lifecycles, a stop signal is sent to the service state watcher
func (m *manager) start() {

	// subscribe to the internal states on behalf of the manager to receive state updates from services.
	statePublishC, unsubscribe := m.iStates.Register(internalServiceStates)
	defer unsubscribe()

	stateWatcherStopC := make(chan struct{}, 1)
	// launch the service state watcher routine.
	doneC := m.serviceStateWatcher(statePublishC, stateWatcherStopC)

	var wg sync.WaitGroup

	var total int
	for _, service := range m.services {
		// service := service                                // rebind loop variable
		service.iStates = m.iStates                       // attach the manager's internal states to each service
		service.Log = m.log.With("service", service.Name) // attach child logger instance with service name

		wg.Add(1)
		// Start each service in its own routine logic / conditional lifecycle.
		go m.startService(&wg, service)
		total++
	}

	m.log.Debug("manager started services", "total", total)
	wg.Wait() // blocks until all services have exited their lifecycles

	// all services have exited their lifecycles at this point.
	close(stateWatcherStopC) // signal to state watcher to stop
	<-doneC                  // wait for state watcher to signal done
	close(m.stateUpdateC)    // close state update publishing channel
	m.log.Debug("manager has stopped running all services")
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
		if !serviceCtx.hasShutdown() {
			wg.Add(1)
			// When shutting down only look for services who are not added as a dependent.
			// This lets us signal shutdown to any parent service or individual service.
			// Parent services will signal shutdown to all their child dependents.
			m.log.Debug("manager signaling stop", "service", serviceCtx.Name)

			// Signal all non-dependent services to shutdown without hanging on for the previous shutdown call.
			go func(svc *ServiceContext) {
				defer wg.Done()
				svc.shutdown()
				<-svc.doneC
			}(serviceCtx)

			totalRunning++
		}
	}

	if totalRunning > 0 {
		m.log.Debug("manager signaled shutdown of running services", "total", totalRunning)
	}
	// wait for all shutdown routine calls to finish.
	m.log.Debug("manager is waiting for all services to shutdown")
	wg.Wait()
	m.log.Debug("manager cancelling context")
	m.cancelCtx()
}

func (m *manager) serviceStateWatcher(statePublishC chan<- States, stopC chan struct{}) <-chan struct{} {
	signalDoneC := make(chan struct{})

	go func() {
		defer close(signalDoneC)

		localStates := make(States)

		for _, sc := range m.services {
			localStates[sc.Name] = InitState
		}

		for {
			select {
			case <-stopC:
				return
			case update, open := <-m.stateUpdateC:
				if !open {
					return
				}
				currState, found := localStates[update.Name]
				if found && currState == update.State {
					// skip any non-changes from previous state.
					continue
				}

				// update the local state
				localStates[update.Name] = update.State

				// make a copy to send out
				states := make(States)
				for name, state := range localStates {
					states[name] = state
				}

				// attempt to publish states to anyone still listening or exit if stop signal received.
				select {
				case <-stopC:
					return
				case statePublishC <- states:
				default:
					// no one was registered to receive the states, drop it.
				}
			}
		}
	}()

	return signalDoneC
}
