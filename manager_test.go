package rxd

import (
	"context"
	"testing"
	"time"
)

type MockService struct {
	mockC chan State
}

func (ms *MockService) Init(sc *ServiceContext) ServiceResponse {
	ms.mockC <- InitState
	return NewResponse(nil, IdleState)
}

func (ms *MockService) Idle(sc *ServiceContext) ServiceResponse {
	ms.mockC <- IdleState
	return NewResponse(nil, RunState)
}

func (ms *MockService) Run(sc *ServiceContext) ServiceResponse {
	ms.mockC <- RunState
	return NewResponse(nil, StopState)
}

func (ms *MockService) Stop(sc *ServiceContext) ServiceResponse {
	defer close(ms.mockC)
	ms.mockC <- StopState
	return NewResponse(nil, ExitState)
}

func TestManagerStart(t *testing.T) {
	vs := &validService{}
	opts := NewServiceOpts()
	validSvc := NewService("valid-service", vs, opts)

	services := []*ServiceContext{validSvc}

	logC := make(chan LogMessage, 10)
	manager := newManager(services)
	manager.setLogCh(logC)

	errC := make(chan error)
	go func() {
		// service should proceed through all lifecycle stages instantly and be done then manager would stop.
		err := manager.start()
		if err != nil {
			errC <- err
		}
	}()

	select {
	case err := <-errC:
		t.Errorf("Manager had an error during run: %s", err)
	case <-manager.stopCh:
		return
	}
}

func TestManagerSetLogC(t *testing.T) {
	services := []*ServiceContext{}

	logC := make(chan LogMessage, 2)

	manager := newManager(services)
	manager.setLogCh(logC)

	go func() {
		logC <- LogMessage{Message: "test", Level: Info}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Errorf("Manager setLogC failed at setting correct log channel due to timeout")
	case logMsg := <-manager.logC:
		if logMsg.Message != "test" || logMsg.Level != Info {
			t.Errorf("Manager setLogC failed at setting correct log channel")
		}
	}
}

func TestManagerStartService(t *testing.T) {
	mock := &MockService{mockC: make(chan State)}
	mockOpts := NewServiceOpts()
	mockSvc := NewService("mock", mock, mockOpts)

	services := []*ServiceContext{}

	logC := make(chan LogMessage, 10)
	manager := newManager(services)
	manager.setLogCh(logC)
	manager.wg.Add(1)
	go manager.startService(mockSvc)
	states := make([]State, 0)

	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()

	for i := 0; i < 5; i++ {
		select {
		case <-timer.C:
			t.Errorf("Manager startService failed due to timeout trying to check state transitions")
		case state := <-mock.mockC:
			if state == "" {
				// close of channel
				break
			}
			states = append(states, state)
		}
		timer.Reset(1 * time.Second)
	}

	if len(states) != 4 {
		t.Errorf("Manager startService failed to transition through all lifecycle states")
	}
}
