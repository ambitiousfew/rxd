package rxd

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ambitiousfew/intracom"
	"golang.org/x/exp/slog"
)

// ServiceContext all services will require a config as a *ServiceContext in their service struct.
// This config contains preconfigured shutdown channel,
type ServiceContext struct {
	Name string
	Ctx  context.Context
	Log  *slog.Logger

	cancelCtx context.CancelFunc

	service Service
	opts    *serviceOpts

	iStates *intracom.Intracom[States]

	stopCalled     atomic.Int32 // 0 = not called, 1 = called
	shutdownCalled atomic.Int32 // 0 = not called, 1 = called

	doneC chan struct{}
}

// NewService creates a new service context instance given a name and options.
func NewService(name string, service Service, opts *serviceOpts) *ServiceContext {
	if opts.logger == nil {
		opts.logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &ServiceContext{
		Ctx:       ctx,
		cancelCtx: cancel,
		Name:      name,
		opts:      opts,

		// 0 = not called, 1 = called
		shutdownCalled: atomic.Int32{},
		service:        service,
		Log:            opts.logger,
		doneC:          make(chan struct{}),
	}
}

// ShutdownSignal returns the channel the side implementing the service should use and watch to be notified
// when the daemon/manager are attempting to shutdown services.
func (sc *ServiceContext) ShutdownSignal() <-chan struct{} {
	return sc.Ctx.Done()
}

func (sc *ServiceContext) hasStopped() bool {
	return sc.stopCalled.Load() == 1
}

func (sc *ServiceContext) shutdown() {
	if sc.shutdownCalled.Swap(1) == 1 {
		return
	}
	sc.cancelCtx() // cancel context to signal shutdown to service
}

// startService is run in its own routine by daemon.
// it is a wrapper around the service's lifecycle methods and
// is responsible for calling the service's lifecycle methods and enforcing the service's run policy.
func startService(wg *sync.WaitGroup, stateUpdateC chan<- StateUpdate, sc *ServiceContext) {
	defer wg.Done()

	// All services begin at Init stage
	var svcResp ServiceResponse = NewResponse(nil, InitState)
	service := sc.service

	for {
		// inform state watcher of this service's upcoming state transition
		stateUpdateC <- StateUpdate{Name: sc.Name, State: svcResp.NextState}

		// Determine the next state the service should be in.
		// Run the method associated with the next state.
		switch svcResp.NextState {

		case InitState:

			svcResp = service.Init(sc)
			if svcResp.Error != nil {
				sc.Log.Error(fmt.Sprintf("%s %s", sc.Name, svcResp.Error.Error()))
			}

		case IdleState:
			svcResp = service.Idle(sc)
			if svcResp.Error != nil {
				sc.Log.Error(fmt.Sprintf("%s %s", sc.Name, svcResp.Error.Error()))
			}

		case RunState:
			svcResp = service.Run(sc)
			if svcResp.Error != nil {
				sc.Log.Error(fmt.Sprintf("%s %s", sc.Name, svcResp.Error.Error()))
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
					sc.stopCalled.Store(1)
					// If Run didnt error, we assume successful run once and stop service.
					svcResp.NextState = ExitState
				}
			}
		case StopState:
			svcResp = service.Stop(sc)
			if svcResp.Error != nil {
				sc.Log.Error(fmt.Sprintf("%s %s", sc.Name, svcResp.Error.Error()))
			}
			sc.stopCalled.Store(1)

		case ExitState:
			if !sc.hasStopped() {
				// inform state watcher of this service's upcoming state transition since we wont loop again after this.
				stateUpdateC <- StateUpdate{Name: sc.Name, State: svcResp.NextState}
				// Ensure we still run Stop in case the user sent us ExitState from any other lifecycle method
				svcResp = service.Stop(sc)
				if svcResp.Error != nil {
					sc.Log.Error(fmt.Sprintf("%s %s", sc.Name, svcResp.Error.Error()))
				}
				sc.stopCalled.Store(1)
			}

			sc.shutdown()
			// we are done with this service, exit the service wrapper routine.
			sc.Log.Debug("service exiting", "service", sc.Name)
			close(sc.doneC)
			return

		default:
			sc.Log.Error(fmt.Sprintf("unknown state '%s' returned from service", svcResp.NextState))
		}

	}
}
