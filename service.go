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

type stageFunc func(*ServiceConfig) ServiceResponse
type Service struct {
	cfg *ServiceConfig

	initFunc   stageFunc
	idleFunc   stageFunc
	runFunc    stageFunc
	stopFunc   stageFunc
	reloadFunc stageFunc
}

func NewService(cfg *ServiceConfig) *Service {
	return &Service{
		cfg: cfg,

		initFunc:   initialize,
		idleFunc:   idle,
		runFunc:    run,
		stopFunc:   stop,
		reloadFunc: reload,
	}
}

func (s *Service) Name() string {
	return s.cfg.name
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
	return s.initFunc(s.cfg)
}

func (s *Service) idle() ServiceResponse {
	return s.idleFunc(s.cfg)
}

func (s *Service) run() ServiceResponse {
	return s.runFunc(s.cfg)
}

func (s *Service) stop() ServiceResponse {
	return s.stopFunc(s.cfg)
}

func (s *Service) reload() ServiceResponse {
	return s.reloadFunc(s.cfg)
}

// Fallback lifecycle stage funcs
func initialize(cfg *ServiceConfig) ServiceResponse {
	for {
		select {
		case <-cfg.ShutdownC:
			return NewResponse(nil, ExitState)
		default:
			return NewResponse(nil, IdleState)
		}
	}
}

func idle(cfg *ServiceConfig) ServiceResponse {
	for {
		select {
		case <-cfg.ShutdownC:
			return NewResponse(nil, ExitState)
		default:
			return NewResponse(nil, RunState)
		}
	}
}

func run(cfg *ServiceConfig) ServiceResponse {
	for {
		select {
		case <-cfg.ShutdownC:
			return NewResponse(nil, ExitState)
		default:
			return NewResponse(nil, StopState)
		}
	}
}

func stop(cfg *ServiceConfig) ServiceResponse {
	for {
		select {
		case <-cfg.ShutdownC:
			return NewResponse(nil, ExitState)
		default:
			return NewResponse(nil, ExitState)
		}
	}
}

func reload(cfg *ServiceConfig) ServiceResponse {
	for {
		select {
		case <-cfg.ShutdownC:
			return NewResponse(nil, ExitState)
		default:
			return NewResponse(nil, InitState)
		}
	}
}
