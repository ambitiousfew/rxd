package rxd

import (
	"testing"
)

type validService struct{}

func (vs *validService) Init(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, IdleState)
}
func (vs *validService) Idle(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, RunState)
}
func (vs *validService) Run(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, StopState)
}
func (vs *validService) Stop(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, ExitState)
}

type invalidService struct{}

func (vs *invalidService) Init(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, IdleState)
}

// missing Idle
func (vs *invalidService) Run(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, StopState)
}
func (vs *invalidService) Stop(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, ExitState)
}

func meetsInterface[T any](i T, n any) bool {
	_, ok := n.(T)
	return ok
}

func TestValidService(t *testing.T) {
	var vs any = &validService{}

	_, ok := vs.(Service)
	if !ok {
		t.Errorf("ValidService did not meet the Service interface to be a valid service")
	}
}

func TestInvalidService(t *testing.T) {
	var ivs any = &invalidService{}

	_, ok := ivs.(Service)

	if ok {
		t.Errorf("InvalidService should not not meet the Service interface but did")
	}
}
