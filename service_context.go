package rxd

import (
	"context"
	"time"
)

type ServiceContext interface {
	context.Context
	Name() string
}

type serviceContext struct {
	context.Context
	name string
}

func NewServiceContext(ctx context.Context, name string) ServiceContext {
	return serviceContext{
		Context: ctx,
		name:    name,
	}
}

func NewServiceContextWithCancel(ctx context.Context, name string) (ServiceContext, context.CancelFunc) {
	sctx, cancel := context.WithCancel(ctx)
	return serviceContext{
		Context: sctx,
		name:    name,
	}, cancel
}

func (sc serviceContext) Name() string {
	return sc.name
}

func (sc serviceContext) Deadline() (deadline time.Time, ok bool) {
	return sc.Context.Deadline()
}

func (sc serviceContext) Done() <-chan struct{} {
	return sc.Context.Done()
}

func (sc serviceContext) Err() error {
	return sc.Context.Err()
}

func (sc serviceContext) Value(key interface{}) interface{} {
	return sc.Context.Value(key)
}

// // NewServiceContext creates a new service context instance given a name, service, and service options.
// func NewService(name string, service Service, opts serviceOpts) *ServiceContext {
// 	if opts == nil {
// 		opts = NewServiceOpts() // if nil is passed, use defaults
// 	}

// 	var log *slog.Logger
// 	if opts.logHandler != nil {
// 		// overrides the logger that manager would attach to the service context
// 		log = slog.New(opts.logHandler).With("service", name)
// 	}

// 	return &ServiceContext{
// 		Name:        name,
// 		ShutdownCtx: opts.ctx,
// 		Log:         log, // if nil, manager takes care of it.

// 		runPolicy: opts.runPolicy,
// 		cancel:    opts.cancel,

// 		// 0 = not called, 1 = called
// 		shutdownCalled: atomic.Int32{},
// 		service:        service,
// 		// attach the service name to the child logger automatically
// 		doneC: make(chan struct{}),
// 	}
// }

// func (sc *ServiceContext) hasStopped() bool {
// 	return sc.stopCalled.Load() == 1
// }

// func (sc *ServiceContext) shutdown() {
// 	if sc.shutdownCalled.Swap(1) == 1 {
// 		return
// 	}
// 	sc.cancel() // cancel context to signal shutdown to service
// }

// // startService is run in its own routine by daemon.
// // it is a wrapper around the service's lifecycle methods and
// // is responsible for calling the service's lifecycle methods and enforcing the service's run policy.
// func startService(wg *sync.WaitGroup, stateUpdateC chan<- StateUpdate, sc *ServiceContext) {
// 	// All services begin at Init stage
// 	var svcResp ServiceResponse = NewResponse(nil, InitState)
// 	service := sc.service

// 	for {
// 		// inform state watcher of this service's upcoming state transition
// 		stateUpdateC <- StateUpdate{Name: sc.Name, State: svcResp.NextState}

// 		// Determine the next state the service should be in.
// 		// Run the method associated with the next state.
// 		switch svcResp.NextState {

// 		case InitState:

// 			svcResp = service.Init(sc)
// 			if svcResp.Error != nil {
// 				sc.Log.Error(fmt.Sprintf("%s %s", sc.Name, svcResp.Error.Error()))
// 			}

// 		case IdleState:
// 			svcResp = service.Idle(sc)
// 			if svcResp.Error != nil {
// 				sc.Log.Error(fmt.Sprintf("%s %s", sc.Name, svcResp.Error.Error()))
// 			}

// 		case RunState:
// 			svcResp = service.Run(sc)
// 			if svcResp.Error != nil {
// 				sc.Log.Error(fmt.Sprintf("%s %s", sc.Name, svcResp.Error.Error()))
// 			}

// 			// Enforce Run policies
// 			switch sc.runPolicy {
// 			case RunOncePolicy:
// 				// regardless of success/fail, we exit
// 				svcResp.NextState = ExitState

// 			case RetryUntilSuccessPolicy:
// 				if svcResp.Error == nil {
// 					svcResp := service.Stop(sc)
// 					if svcResp.Error != nil {
// 						continue
// 					}
// 					sc.stopCalled.Store(1)
// 					// If Run didnt error, we assume successful run once and stop service.
// 					svcResp.NextState = ExitState
// 				}
// 			}
// 		case StopState:
// 			svcResp = service.Stop(sc)
// 			if svcResp.Error != nil {
// 				sc.Log.Error(fmt.Sprintf("%s %s", sc.Name, svcResp.Error.Error()))
// 			}
// 			sc.stopCalled.Store(1)

// 		case ExitState:
// 			if !sc.hasStopped() {
// 				// inform state watcher of this service's upcoming state transition since we wont loop again after this.
// 				stateUpdateC <- StateUpdate{Name: sc.Name, State: svcResp.NextState}
// 				// Ensure we still run Stop in case the user sent us ExitState from any other lifecycle method
// 				svcResp = service.Stop(sc)
// 				if svcResp.Error != nil {
// 					sc.Log.Error(fmt.Sprintf("%s %s", sc.Name, svcResp.Error.Error()))
// 				}
// 				sc.stopCalled.Store(1)
// 			}

// 			sc.shutdown()
// 			// we are done with this service, exit the service wrapper routine.
// 			sc.Log.Debug("service exiting")
// 			close(sc.doneC)

// 			wg.Done()
// 			return

// 		default:
// 			sc.Log.Error(fmt.Sprintf("unknown state '%s' returned from service", svcResp.NextState))
// 		}
// 	}
// }
