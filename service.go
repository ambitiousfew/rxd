package rxd

// State is used to determine the "next state" the service should enter
// when the current state has completed/errored returned. State should
// reflect different states that the interface can enter.
type State string

const (
	// InitState is in the ServiceResponse to inform manager to move us to the Init state (Initial Default).
	InitState State = "init"
	// IdleState is in the ServiceResponse to inform manager to move us to the Idle state
	IdleState State = "idle"
	// RunState is in the ServiceResponse to inform manager to move us to the Run state
	RunState State = "run"
	// StopState is in the ServiceResponse to inform manager to move us to the Stop state
	StopState State = "stop"
	// ExitState is in the ServiceResponse to inform manager to act as the final response type for Stop.
	ExitState State = "exit"
)

type stageFunc func(*ServiceContext) ServiceResponse
type Service struct {
	ctx *ServiceContext

	initFunc   stageFunc
	idleFunc   stageFunc
	runFunc    stageFunc
	stopFunc   stageFunc
	reloadFunc stageFunc
}

func NewService(ctx *ServiceContext) *Service {
	return &Service{
		ctx:        ctx,
		initFunc:   initialize,
		idleFunc:   idle,
		runFunc:    run,
		stopFunc:   stop,
		reloadFunc: reload,
	}
}

func (s *Service) Name() string {
	return s.ctx.name
}

func (s *Service) UsingInitFunc(f stageFunc) {
	s.initFunc = f
}

func (s *Service) UsingIdleFunc(f stageFunc) {
	s.idleFunc = f
}

func (s *Service) UsingRunFunc(f stageFunc) {
	s.runFunc = f
}

func (s *Service) UsingStopFunc(f stageFunc) {
	s.stopFunc = f
}

func (s *Service) UsingReloadFunc(f stageFunc) {
	s.reloadFunc = f
}

func (s *Service) init() ServiceResponse {
	return s.initFunc(s.ctx)
}

func (s *Service) idle() ServiceResponse {
	return s.idleFunc(s.ctx)
}

func (s *Service) run() ServiceResponse {
	return s.runFunc(s.ctx)
}

func (s *Service) stop() ServiceResponse {
	return s.stopFunc(s.ctx)
}

func (s *Service) reload() ServiceResponse {
	return s.reloadFunc(s.ctx)
}

// Fallback lifecycle stage funcs
func initialize(ctx *ServiceContext) ServiceResponse {
	for {
		select {
		case <-ctx.shutdownC:
			return NewResponse(nil, ExitState)
		default:
			return NewResponse(nil, IdleState)
		}
	}
}

func idle(ctx *ServiceContext) ServiceResponse {
	for {
		select {
		case <-ctx.shutdownC:
			return NewResponse(nil, ExitState)
		default:
			return NewResponse(nil, RunState)
		}
	}
}

func run(ctx *ServiceContext) ServiceResponse {
	for {
		select {
		case <-ctx.shutdownC:
			return NewResponse(nil, ExitState)
		default:
			return NewResponse(nil, StopState)
		}
	}
}

func stop(ctx *ServiceContext) ServiceResponse {
	for {
		select {
		case <-ctx.shutdownC:
			return NewResponse(nil, ExitState)
		default:
			return NewResponse(nil, ExitState)
		}
	}
}

func reload(ctx *ServiceContext) ServiceResponse {
	for {
		select {
		case <-ctx.shutdownC:
			return NewResponse(nil, ExitState)
		default:
			return NewResponse(nil, InitState)
		}
	}
}
