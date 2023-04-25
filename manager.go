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

func (m *manager) startService(serviceCtx *ServiceContext) {
	defer m.wg.Done()

	serviceCtx.setLogChannel(m.logC)

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
			m.logC <- NewLog(fmt.Sprintf("%s next state, init", serviceCtx.name), Debug)
			svcResp = service.Init(serviceCtx)

		case IdleState:
			m.logC <- NewLog(fmt.Sprintf("%s next state, idle", serviceCtx.name), Debug)
			svcResp = service.Idle(serviceCtx)

		case RunState:
			m.logC <- NewLog(fmt.Sprintf("%s next state, run", serviceCtx.name), Debug)
			svcResp = service.Run(serviceCtx)
			// Enforce Run policies
			switch serviceCtx.opts.runPolicy {
			case RunOncePolicy:
				if svcResp.Error != nil {
					m.logC <- NewLog(svcResp.Error.Error(), Error)
				}
				// regardless of success/fail, we exit
				svcResp.NextState = ExitState

			case RetryUntilSuccessPolicy:
				if svcResp.Error == nil {
					stopResp := service.Stop(serviceCtx)
					if stopResp.Error != nil {
						m.logC <- NewLog(stopResp.Error.Error(), Error)
					}
					serviceCtx.isStopped = true
					// If Run didnt error, we assume successful run once and stop service.
					svcResp.NextState = ExitState
				}
			}

		case StopState:
			m.logC <- NewLog(fmt.Sprintf("%s next state, stop", serviceCtx.name), Debug)
			svcResp = service.Stop(serviceCtx)
			if svcResp.Error != nil {
				m.logC <- NewLog(svcResp.Error.Error(), Error)
			}

			serviceCtx.setIsStopped(true)
			return
		}

		// No matter what stage ran above, whatever it returned for a ServiceResponse
		// Extract the error out and send it to the logging channel.
		if svcResp.Error != nil {
			m.logC <- NewLog(svcResp.Error.Error(), Error)
			continue
		}

		if svcResp.NextState == ExitState {
			// establish lock before reading/writing isStopped/isShutdown
			serviceCtx.LogDebug("entering ExitState")
			serviceCtx.notifyStateChange(StopState)

			if !serviceCtx.isStopped {
				// Ensure we still run Stop in case the user sent us ExitState from any other lifecycle method
				svcResp = service.Stop(serviceCtx)
				if svcResp.Error != nil {
					m.logC <- NewLog(svcResp.Error.Error(), Error)
				}
				serviceCtx.setIsStopped(true)
			}

			serviceCtx.LogDebug("shutting down")
			// if a close signal hasnt been sent to the service.
			serviceCtx.shutdown()
			return
		}
	}
}

// start handles launching each service in its own routine
// it blocks on the MAIN routine until all services have finished running and
// notified WaitGroup by calling .Done()
// The MAIN routine returns control back to the Daemon to finish running.
func (m *manager) start() (exitErr error) {
	defer func() {
		// capture any panics, convert to error to return
		if rErr := recover(); rErr != nil {
			exitErr = fmt.Errorf("%s", rErr)
		}
	}()

	go func() {
		// Watch for stop signal, perform shutdown
		m.logC <- NewLog("manager watching for stop signal....", Debug)
		<-m.stopCh
		m.logC <- NewLog("manager received stop signal", Debug)
		// iterates over all service goroutines to signal them to shutdown.
		m.shutdown()
	}()

	for _, service := range m.services {
		m.wg.Add(1)
		// Start each service in its own routine logic / conditional lifecycle.
		go m.startService(service)
	}

	m.logC <- NewLog("Started all services...", Info)

	// Main thread blocking forever infinite loop to select between
	//  listening for OS Signal and/or errors to print from each service.
	m.wg.Wait()

	m.logC <- NewLog("All services have stopped running", Info)
	// signal complete using context
	m.cancelCtx()
	return exitErr
}

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

	m.logC <- NewLog(fmt.Sprintf("%d running services have been signaled to shut down.", totalRunning), Debug)
}
