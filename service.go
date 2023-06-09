package rxd

import (
	"context"
	"log"
	"sync/atomic"
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
		opts.logger = NewLogger(LevelInfo, log.LstdFlags|log.Lmsgprefix|log.Lshortfile)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &ServiceContext{
		Ctx:          ctx,
		cancelCtx:    cancel,
		Name:         name,
		stateC:       make(chan State),
		stateChangeC: make(chan State),
		opts:         opts,
		isStopped:    true,

		// 0 = not called, 1 = called
		shutdownCalled: atomic.Int32{},
		service:        service,
		dependents:     make(map[*ServiceContext]map[State]struct{}),
		Log:            opts.logger,
	}
}
