package rxdaemon

// ServiceResponse is used as a return response for services to use
// so the Manager can determine the "next state" based on return and any error
// that needs to be broadcasted up the logging channel
type ServiceResponse struct {
	Error     error
	NextState State
}

// NewResponse will create a new ServiceResponse given an error and next state.
func NewResponse(err error, nextState State) ServiceResponse {
	return ServiceResponse{
		Error:     err,
		NextState: nextState,
	}
}
