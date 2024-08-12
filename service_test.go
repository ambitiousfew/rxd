package rxd

import (
	"testing"
	"time"

	"github.com/ambitiousfew/rxd/log"
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
	timer        *time.Timer
	stateTimeout time.Duration
}

func newMockService(stateTimeout time.Duration) *mockService {
	return &mockService{
		stateTimeout: stateTimeout,
		timer:        time.NewTimer(stateTimeout),
	}
}

func (m *mockService) Init(sctx ServiceContext) error {
	sctx.Log(log.LevelInfo, "mockService.Init")
	m.timer.Reset(m.stateTimeout)

	select {
	case <-sctx.Done():
		return nil
	case <-m.timer.C:
		sctx.Log(log.LevelInfo, "timer expired moving to idle state")
		return nil
	}
}

func (m *mockService) Idle(sctx ServiceContext) error {
	sctx.Log(log.LevelInfo, "mockService.Idle")
	m.timer.Reset(m.stateTimeout)
	select {
	case <-sctx.Done():
		return nil
	case <-m.timer.C:
		return nil
	}
}

func (m *mockService) Run(sctx ServiceContext) error {
	sctx.Log(log.LevelInfo, "mockService.Run")
	m.timer.Reset(m.stateTimeout)

	select {
	case <-sctx.Done():
		return nil
	case <-m.timer.C:
		return nil
	}
}

func (m *mockService) Stop(sctx ServiceContext) error {
	sctx.Log(log.LevelInfo, "mockService.Stop")
	m.timer.Reset(m.stateTimeout)

	select {
	case <-sctx.Done():
		return nil
	case <-m.timer.C:
		return nil
	}
}
