package rxdaemon

import (
	"fmt"
	"sync"
	"time"
)

type manager struct {
	logC     chan LogMessage
	stoppedC chan struct{}

	Services []Service
}

func (m *manager) startService(service Service, wg *sync.WaitGroup) {
	defer wg.Done()

	svcCfg := service.Config()
	svcCfg.isStopped = false
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
				// m.logger.Info.Println("next state, init")
				m.logC <- NewLog(fmt.Sprintf("%s next state, init", service.Name()), Debug)
				svcResp = service.Init()
			case IdleState:
				// m.logger.Info.Println("next state, idle")
				m.logC <- NewLog(fmt.Sprintf("%s next state, idle", service.Name()), Debug)
				svcResp = service.Idle()
			case RunState:
				// m.logger.Info.Println("next state, run")
				m.logC <- NewLog(fmt.Sprintf("%s next state, run", service.Name()), Debug)
				svcResp = service.Run()
			case StopState:
				// m.logger.Info.Println("next state, stop")
				m.logC <- NewLog(fmt.Sprintf("%s next state, stop", service.Name()), Debug)
				svcResp = service.Stop()
				return
			case NoopState:
				// Probably not necessary to keep around, really meant for Stop() to use as ServiceResponse
				// Debating on not using ServiceResponse for stop, just using classic error
				m.logC <- NewLog(fmt.Sprintf("%s next state, noop", service.Name()), Debug)
				return

			default:
				// Shouldn't be possible to end up here. Fallback is to end the service.
				return
			}

			// No matter what stage ran above, whatever it returned for a ServiceResponse
			// Extract the error out and send it to the logging channel.
			if svcResp.Error != nil {
				m.logC <- NewLog(svcResp.Error.Error(), Error)

				// Figure out if we should continue to run or not
				switch svcCfg.Opts.RestartPolicy {
				case Always:
					m.logC <- NewLog(fmt.Sprintf("%s %s failed, retrying in %.2f seconds...\n", service.Name(), svcResp.NextState, svcCfg.Opts.RestartTimeout.Seconds()), Error)
					time.Sleep(svcCfg.Opts.RestartTimeout)
					continue
				default:
					// Currently the only other option is restart policy of: Once
					// If this service is only meant to run once. Prevent a reiteration/restart.
					svcResp := service.Stop()
					if svcResp.Error != nil {
						m.logC <- NewLog(svcResp.Error.Error(), Error)
					}
					// m.logger.Info.Printf("%s has completed successfully, stopping service.\n", svc.Name())
					m.logC <- NewLog(fmt.Sprintf("%s has completed successfully, stopping service.\n", service.Name()), Debug)
					return
				}
			}

		}
	}

}

// Start handles launching each service in its own routine
// it blocks on the MAIN routine until all services have finished running and
// notified WaitGroup by calling .Done()
// The MAIN routine returns control back to the Daemon to finish running.
func (m *manager) Start() error {
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

	return nil
}

func (m *manager) shutdown() error {
	for _, service := range m.Services {
		svcCfg := service.Config()
		if !svcCfg.isStopped {
			m.logC <- NewLog(fmt.Sprintf("Signaling stop of service: %s\n", service.Name()), Debug)
			// sends a signal to each service to inform them to stop running.
			close(svcCfg.ShutdownC)
		}
	}

	m.logC <- NewLog("All services shut down.", Debug)

	// sends a close signal back up to inform daemon it has finished.
	close(m.stoppedC)
	return nil
}
