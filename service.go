package rxd

import "context"

// ServiceState represents the possible states a service can be in.
type ServiceState int

func (s ServiceState) String() string {
	switch s {
	case unknown:
		return "unknown"
	case setup:
		return "setup"
	case Init:
		return "init"
	case Idle:
		return "idle"
	case Run:
		return "run"
	case Stop:
		return "stop"
	case Exit:
		return "exit"
	default:
		return "unknown"
	}
}

const (
	// unexported to prevent user selecting states that aren't lifecycles.
	unknown ServiceState = iota
	setup

	Init
	Idle
	Run
	Stop
	Exit
)

func NewResponse(err error, state ServiceState) ServiceResponse {
	return ServiceResponse{
		Next: state,
		Err:  err,
	}
}

// ServiceResponse is used by services to indicate their next desired state.
type ServiceResponse struct {
	Next ServiceState
	Err  error
}

// Service is the interface that all services must implement.
type Service interface {
	Setup(context.Context) ServiceConfig
	Init(context.Context) ServiceResponse
	Idle(context.Context) ServiceResponse
	Run(context.Context) ServiceResponse
	Stop(context.Context) ServiceResponse
}
