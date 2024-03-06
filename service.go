package rxd

// NewResponse is a helper function to create a new ServiceResponse
// where a ServiceResponse represents the next lifecycle state the caller
// wishes to move to and any error that occurred during the current state.
func NewResponse(err error, state ServiceState) ServiceResponse {
	return ServiceResponse{
		Next: state,
		Err:  err,
	}
}

// ServiceResponse is used by service lifecycle methods to indicate their next desired state
// and any errors that occurred during the current state.
type ServiceResponse struct {
	Next ServiceState
	Err  error
}

type Service struct {
	Conf ServiceConfig
	Svc  Servicer
}

func NewService(name string, svc Servicer, options ...ServiceOption) Service {
	return Service{
		Conf: ServiceConfig{
			Name:    name,
			RunOpts: options,
		},
		Svc: svc,
	}
}

// Servicer is the interface that all services must implement.
type Servicer interface {
	Init(ServiceContext) ServiceResponse
	Idle(ServiceContext) ServiceResponse
	Run(ServiceContext) ServiceResponse
	Stop(ServiceContext) ServiceResponse
}
