package rxd

import "context"

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

type Service struct {
	Conf ServiceConfig
	Svc  Servicer
}

// Servicer is the interface that all services must implement.
type Servicer interface {
	Init(context.Context) ServiceResponse
	Idle(context.Context) ServiceResponse
	Run(context.Context) ServiceResponse
	Stop(context.Context) ServiceResponse
}
