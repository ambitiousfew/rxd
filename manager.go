package rxd

import (
	"fmt"
	"sync"
)

type manager struct {
	logC     chan LogMessage
	stoppedC chan struct{}

	Services []Service
}

func (m *manager) startService(service Service, wg *sync.WaitGroup) {
	defer wg.Done()

	svcCfg := service.Config()
	// svcCfg.isStopped = false
	// svcCfg.isShutdown = false
	// Attach manager logging channel to each service so all services can send logs out
	svcCfg.logC = m.logC

	// All services begin at Init stage
	var svcResp ServiceResponse = NewResponse(nil, InitState)

	for {
		select {
		case <-svcCfg.ShutdownC:
			m.logC <- NewLog(fmt.Sprintf("%s received shutdown signal", service.Name()), Debug)
			return

		default:
			// Determine the next state the service should be in.
			// Run the method associated with the next state.
			switch svcResp.NextState {

			case InitState:
				m.logC <- NewLog(fmt.Sprintf("%s next state, init", service.Name()), Debug)
				svcResp = service.Init()

			case IdleState:
				m.logC <- NewLog(fmt.Sprintf("%s next state, idle", service.Name()), Debug)
				svcResp = service.Idle()

			case RunState:
				m.logC <- NewLog(fmt.Sprintf("%s next state, run", service.Name()), Debug)
				svcResp = service.Run()
				// Enforce Run policies
				switch svcCfg.opts.runPolicy {
				case RunOncePolicy:
					if svcResp.Error != nil {
						m.logC <- NewLog(svcResp.Error.Error(), Error)
					}
					// regardless of success/fail, we exit
					svcResp.NextState = ExitState

				case RetryUntilSuccessPolicy:
					if svcResp.Error == nil {
						stopResp := service.Stop()
						if stopResp.Error != nil {
							m.logC <- NewLog(stopResp.Error.Error(), Error)
						}
						svcCfg.isStopped = true
						// If Run didnt error, we assume successful run once and stop service.
						svcResp.NextState = ExitState
					}
				}

			case StopState:
				m.logC <- NewLog(fmt.Sprintf("%s next state, stop", service.Name()), Debug)
				svcResp = service.Stop()
				if svcResp.Error != nil {
					m.logC <- NewLog(svcResp.Error.Error(), Error)
				}

				svcCfg.setIsStopped(true)
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

				if !svcCfg.isStopped {
					// Ensure we still run Stop in case the user sent us ExitState from any other lifecycle method
					svcResp = service.Stop()
					if svcResp.Error != nil {
						m.logC <- NewLog(svcResp.Error.Error(), Error)
					}
					svcCfg.setIsStopped(true)
				}

				// if a close signal hasnt been sent to the service.
				if !svcCfg.isShutdown {
					m.logC <- NewLog(fmt.Sprintf("sending a close signal to %s", service.Name()), Error)
					close(svcCfg.ShutdownC)
					close(svcCfg.StateC)
					svcCfg.setIsShutdown(true)
				}

				m.logC <- NewLog(fmt.Sprintf("%s is exiting...", service.Name()), Debug)
				return
			}
		}
	}

}

// start handles launching each service in its own routine
// it blocks on the MAIN routine until all services have finished running and
// notified WaitGroup by calling .Done()
// The MAIN routine returns control back to the Daemon to finish running.
func (m *manager) start() error {
	var err error
	defer func() error {
		if err != nil {
			return err
		}
		// catch any potential panics from manager to return to daemon
		err = recover().(error)
		return err
	}()

	var wg sync.WaitGroup
	for _, service := range m.Services {
		wg.Add(1)
		// Start each service in its own routine logic / conditional lifecycle.
		go m.startService(service, &wg)
	}

	m.logC <- NewLog("Started all services...", Info)

	// Main thread blocking forever infinite loop to select between
	//  listening for OS Signal and/or errors to print from each service.
	wg.Wait()

	m.logC <- NewLog("All services have stopped running", Info)
	return nil
}

func (m *manager) shutdown() {
	var totalRunning int
	for _, service := range m.Services {
		svcCfg := service.Config()

		if !svcCfg.isShutdown {
			m.logC <- NewLog(fmt.Sprintf("Signaling stop of service: %s", service.Name()), Debug)
			// sends a signal to each service to inform them to stop running.
			close(svcCfg.ShutdownC)
			svcCfg.setIsShutdown(true)
			close(svcCfg.StateC)
			totalRunning++
		}
	}

	m.logC <- NewLog(fmt.Sprintf("%d running services have been signaled to shut down.", totalRunning), Debug)

	// sends a close signal back up to inform daemon it has finished.
	close(m.stoppedC)
}
