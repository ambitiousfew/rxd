package rxd

import (
	"sync/atomic"

	"golang.org/x/exp/slog"
)

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

type Service interface {
	Init(*ServiceContext) ServiceResponse
	Idle(*ServiceContext) ServiceResponse
	Run(*ServiceContext) ServiceResponse
	Stop(*ServiceContext) ServiceResponse
}

// NewService creates a new service instance given a name and options.
func NewService(name string, service Service, opts *serviceOpts) *ServiceContext {
	if opts.logger == nil {
		opts.logger = slog.Default()
	}

	return &ServiceContext{
		Name: name,
		opts: opts,

		cancel: make(chan struct{}),

		isStopped:  atomic.Int32{}, // 0 = not stopped, 1 = stopped
		isShutdown: atomic.Int32{}, // 0 = not shutdown, 1 = shutdown

		service: service,
		Log:     opts.logger,
	}
}
