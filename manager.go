package rxd

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ambitiousfew/intracom"
	"golang.org/x/exp/slog"
)

type manager struct {
	services []*ServiceContext

	intracom *intracom.Intracom[[]byte]
	log      *slog.Logger

	stateUpdateC   chan StateUpdate
	shutdownCalled atomic.Int32
}

// setLogger sets the passed in logging instance as the logger for manager and all services within manager.
func (m *manager) setLogger(logger *slog.Logger) {
	m.log = logger
}

// startService is run by manager in its own routine
// its a service wrapper so lifecycle stages can be controlled
// and relayed behind the scenes.
func (m *manager) startService(sc *ServiceContext) {

	// All services begin at Init stage
	var svcResp ServiceResponse = NewResponse(nil, InitState)
	service := sc.service // remap for readability

	for {
		m.log.Debug("rxd manager publishing state update", "service", sc.Name)
		m.stateUpdateC <- StateUpdate{Name: sc.Name, State: svcResp.NextState}
		m.log.Debug("rxd manager done publishing state update", "service", sc.Name)
		// Every service attempts to notify any services that were set during setup via UsingServiceNotify option.
		// Determine the next state the service should be in.
		// Run the method associated with the next state.
		switch svcResp.NextState {

		case InitState:
			m.log.Debug("rxd manager is initializing service", "service", sc.Name)
			svcResp = service.Init(sc)
			if svcResp.Error != nil {
				sc.Log.Error(svcResp.Error.Error(), "service", sc.Name)
			}

		case IdleState:
			m.log.Debug("rxd manager is idling service", "service", sc.Name)
			svcResp = service.Idle(sc)
			if svcResp.Error != nil {
				sc.Log.Error(svcResp.Error.Error(), "service", sc.Name)
			}

		case RunState:
			m.log.Debug("rxd manager is running service", "service", sc.Name)
			svcResp = service.Run(sc)
			if svcResp.Error != nil {
				sc.Log.Error(svcResp.Error.Error(), "service", sc.Name)
			}

			// Enforce Run policies
			switch sc.opts.runPolicy {
			case RunOncePolicy:
				// regardless of success/fail, we exit
				svcResp.NextState = ExitState

			case RetryUntilSuccessPolicy:
				if svcResp.Error == nil {
					svcResp := service.Stop(sc)
					if svcResp.Error != nil {
						continue
					}
					sc.isStopped.Store(1)
					// If Run didnt error, we assume successful run once and stop service.
					svcResp.NextState = ExitState
				}
			}
		case StopState:
			m.log.Debug("rxd manager is stopping service", "service", sc.Name)
			svcResp = service.Stop(sc)
			if svcResp.Error != nil {
				sc.Log.Error(svcResp.Error.Error(), "service", sc.Name)
			}
			sc.isStopped.Store(1)

			// TODO: consider the ability to stop a service and restart it instead of forcing exit.
			svcResp.NextState = ExitState

		case ExitState:
			m.log.Debug("rxd manager is exiting service", "service", sc.Name)
			if !sc.hasStopped() {
				// Ensure we still run Stop in case the user sent us ExitState from any other lifecycle method
				svcResp = service.Stop(sc)
				if svcResp.Error != nil {
					sc.Log.Error(svcResp.Error.Error(), "service", sc.Name)
				}
				sc.isStopped.Store(1)
			}

			<-sc.shutdown() // wait for service to finish its shutdown routine
			m.log.Debug("rxd manager received shutdown signal", "service", sc.Name)
			return
		}
	}

}

func (m *manager) preStartCheck() error {
	serviceNames := make(map[string]struct{})
	for _, service := range m.services {
		if _, exists := serviceNames[service.Name]; exists {
			return fmt.Errorf("rxd manager error: the service named '%s' overlaps with another service name", service.Name)
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
func (m *manager) start() {
	stopC := make(chan struct{}) // used to signal state watcher to stop
	doneC := m.serviceStateWatcher(stopC)

	var wg sync.WaitGroup

	// manager is reponsible for starting each service in its own routine
	for _, service := range m.services {
		service := service
		service.setLogger(m.log)        // ensure each service receives logger instance for logging.
		service.setIntracom(m.intracom) // ensure each service receives intracom instance for intra-comms.

		wg.Add(1)
		// each service starts in their own routine
		go func() {
			defer wg.Done()
			m.startService(service) // blocks until service is done running its lifecycle
		}()
	}

	m.log.Debug("rxd manager started all services", "total", len(m.services))

	wg.Wait() // blocks until all services are done running their lifecycles

	close(stopC) // signal state watcher to stop
	<-doneC      // wait for state watcher to signal done

	// all services are done running their lifecycles beyond this point.
	m.log.Debug("rxd manager has stopped running all services")
}

// shutdown handles iterating over any remaining services that still may be running
// calling each services shutdown method if it hasnt already been called by the service itself
// on a manual Stop/Exit state before manager began its own shutdown.
func (m *manager) shutdown() {
	if m.shutdownCalled.Swap(1) == 1 {
		// if the old val is already 1, then shutdown has already been called before, dont run twice.
		return
	}
	var wg sync.WaitGroup

	var totalRunning int
	for _, service := range m.services {
		service := service // rebind loop variable
		if !service.hasShutdown() {
			wg.Add(1)

			go func() {
				defer wg.Done()
				m.log.Debug("rxd manager is signaling stop", "service", service.Name)
				<-service.shutdown() // wait for service to finish its shutdown routine
				m.log.Debug("rxd manager done signaling service stop", "name", service.Name)
			}()

			totalRunning++
		}
	}

	if totalRunning > 0 {
		m.log.Debug("rxd manager has signaled shutdown to all services", "total", totalRunning)
	}
	// wait for all shutdown routine calls to finish.
	m.log.Debug("rxd manager is waiting for all services to shutdown")
	wg.Wait()
	m.log.Debug("rxd manager is done wating for all services to shutdown")
}

func (m *manager) serviceStateWatcher(signalStopC <-chan struct{}) <-chan struct{} {
	doneC := make(chan struct{})

	go func() {
		publishStateC, unregister := m.intracom.Register(internalServiceStates)
		defer close(doneC)
		defer unregister()

		states := make(map[string]State)

		for {
			select {
			case <-signalStopC:
				m.log.Debug("rxd manager state watcher received stop signal")
				return
			case update := <-m.stateUpdateC:
				currState, found := states[update.Name]
				if found && currState == update.State {
					continue // skip any non-changes from previous state.
				}

				states[update.Name] = update.State

				statesBytes, err := json.Marshal(states)
				if err != nil {
					m.log.Debug(fmt.Sprintf("rxd manager error marshalling updated states: %s", err))
					continue
				}

				select {
				case <-signalStopC:
					return
				case publishStateC <- statesBytes:
				}
			}
		}
	}()

	return doneC
}
