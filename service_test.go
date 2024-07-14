package rxd

import (
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	name := "test-mock-service"

	mockService := newMockService(100 * time.Millisecond)
	service := NewService(name, mockService)

	if service.Name != name {
		t.Errorf("Expected service.Name to be %s, got %s", name, service.Name)
	}

	if service.Runner != mockService {
		t.Errorf("Expected service.Runner to be %v, got %v", mockService, service.Runner)
	}

	if _, ok := service.Handler.(DefaultHandler); !ok {
		t.Errorf("Expected service.Handler to be DefaultHandler{}, got %v", service.Handler)
	}
}

func TestNewServiceWithHandler(t *testing.T) {
	name := "test-mock-service"

	mockService := newMockService(100 * time.Millisecond)
	mockHandler := mockServiceHandler{}
	service := NewService(name, mockService, WithHandler(mockHandler))

	if service.Name != name {
		t.Errorf("Expected service.Name to be %s, got %s", name, service.Name)
	}

	if service.Runner != mockService {
		t.Errorf("Expected service.Runner to be %v, got %v", mockService, service.Runner)
	}

	if _, ok := service.Handler.(mockServiceHandler); !ok {
		t.Errorf("Expected service.Handler to be a mockServiceHandler{}, got %v", service.Handler)
	}

}

type mockService struct {
	TransitionTimeout time.Duration
}

func newMockService(stateTimeout time.Duration) *mockService {
	return &mockService{
		TransitionTimeout: stateTimeout,
	}
}

func (m *mockService) Init(sctx ServiceContext) (State, error) {
	ticker := time.NewTicker(m.TransitionTimeout)
	defer ticker.Stop()

	select {
	case <-sctx.Done():
		return StateExit, nil
	case <-ticker.C:
		return StateIdle, nil
	}
}

func (m *mockService) Idle(sctx ServiceContext) (State, error) {
	ticker := time.NewTicker(m.TransitionTimeout)
	defer ticker.Stop()

	select {
	case <-sctx.Done():
		return StateExit, nil
	case <-ticker.C:
		return StateRun, nil
	}
}

func (m *mockService) Run(sctx ServiceContext) (State, error) {
	ticker := time.NewTicker(m.TransitionTimeout)
	defer ticker.Stop()

	select {
	case <-sctx.Done():
		return StateExit, nil
	case <-ticker.C:
		return StateStop, nil
	}
}

func (m *mockService) Stop(sctx ServiceContext) (State, error) {
	ticker := time.NewTicker(m.TransitionTimeout)
	defer ticker.Stop()

	select {
	case <-sctx.Done():
		return StateExit, nil
	case <-ticker.C:
		return StateInit, nil
	}
}
