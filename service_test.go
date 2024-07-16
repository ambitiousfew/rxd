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

	if _, ok := service.Manager.(RunContinuousManager); !ok {
		t.Errorf("Expected service.Handler to be DefaultHandler{}, got %v", service.Manager)
	}
}

func TestNewServiceWithHandler(t *testing.T) {
	name := "test-mock-service"

	mockService := newMockService(100 * time.Millisecond)
	mockManager := mockServiceManager{}
	service := NewService(name, mockService, WithManager(mockManager))

	if service.Name != name {
		t.Errorf("Expected service.Name to be %s, got %s", name, service.Name)
	}

	if service.Runner != mockService {
		t.Errorf("Expected service.Runner to be %v, got %v", mockService, service.Runner)
	}

	if _, ok := service.Manager.(mockServiceManager); !ok {
		t.Errorf("Expected service.Handler to be a mockServiceHandler{}, got %v", service.Manager)
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

func (m *mockService) Init(sctx ServiceContext) error {
	ticker := time.NewTicker(m.TransitionTimeout)
	defer ticker.Stop()

	select {
	case <-sctx.Done():
		return nil
	case <-ticker.C:
		return nil
	}
}

func (m *mockService) Idle(sctx ServiceContext) error {
	ticker := time.NewTicker(m.TransitionTimeout)
	defer ticker.Stop()

	select {
	case <-sctx.Done():
		return nil
	case <-ticker.C:
		return nil
	}
}

func (m *mockService) Run(sctx ServiceContext) error {
	ticker := time.NewTicker(m.TransitionTimeout)
	defer ticker.Stop()

	select {
	case <-sctx.Done():
		return nil
	case <-ticker.C:
		return nil
	}
}

func (m *mockService) Stop(sctx ServiceContext) error {
	ticker := time.NewTicker(m.TransitionTimeout)
	defer ticker.Stop()

	select {
	case <-sctx.Done():
		return nil
	case <-ticker.C:
		return nil
	}
}
