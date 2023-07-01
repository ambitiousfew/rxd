package rxd

import (
	"log"
	"testing"
)

type TestNoOpLogger struct {
	debug *log.Logger
	info  *log.Logger
	warn  *log.Logger
	err   *log.Logger
}

func (tl *TestNoOpLogger) Debug(v ...any)                 {}
func (tl *TestNoOpLogger) Debugf(format string, v ...any) {}
func (tl *TestNoOpLogger) Info(v ...any)                  {}
func (tl *TestNoOpLogger) Infof(format string, v ...any)  {}
func (tl *TestNoOpLogger) Warn(v ...any)                  {}
func (tl *TestNoOpLogger) Warnf(format string, v ...any)  {}
func (tl *TestNoOpLogger) Error(v ...any)                 {}
func (tl *TestNoOpLogger) Errorf(format string, v ...any) {}

type MockService struct{}

func (ms *MockService) Init(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, IdleState)
}

func (ms *MockService) Idle(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, RunState)
}

func (ms *MockService) Run(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, StopState)
}

func (ms *MockService) Stop(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, ExitState)
}

func TestManagerStart(t *testing.T) {
	vs := &validService{}
	opts := NewServiceOpts()
	validSvc := NewService("valid-service", vs, opts)

	services := []*ServiceContext{validSvc}

	manager := newManager(services)
	manager.setLogger(&TestNoOpLogger{})

	stopWatcherC := make(chan struct{})
	go func() {
		manager.serviceStateWatcher(stopWatcherC)
	}()

	errC := make(chan error)
	go func() {
		// service should proceed through all lifecycle stages instantly and be done then manager would stop.
		err := manager.start()
		// if manager start finishes, signal stop of watcher
		close(stopWatcherC)
		if err != nil {
			errC <- err
		}
	}()

	select {
	case err := <-errC:
		t.Errorf("manager had an error during run: %s", err)
	case <-manager.stopCh:
		return
	}
}

// TODO: Fix this test to use intracom in-place of the mock channel test.
func TestManagerGoodPreStartCheck(t *testing.T) {
	mock1 := &MockService{}
	mock1Opts := NewServiceOpts()
	mock1Svc := NewService("mock-1", mock1, mock1Opts)

	mock2 := &MockService{}
	mock2Opts := NewServiceOpts()
	mock2Svc := NewService("mock-2", mock2, mock2Opts)

	services := []*ServiceContext{mock1Svc, mock2Svc}

	manager := newManager(services)
	manager.setLogger(&TestNoOpLogger{})

	manager.preStartCheck()
	if err := manager.preStartCheck(); err != nil {
		t.Errorf("wanted nil error, got %s", err)
	}
}

func TestManagerBadPreStartCheck(t *testing.T) {
	mock1 := &MockService{}
	mock1Opts := NewServiceOpts()
	mock1Svc := NewService("mock-1", mock1, mock1Opts)

	mock2 := &MockService{}
	mock2Opts := NewServiceOpts()
	mock2Svc := NewService("mock-1", mock2, mock2Opts)

	services := []*ServiceContext{mock1Svc, mock2Svc}

	manager := newManager(services)
	manager.setLogger(&TestNoOpLogger{})

	if err := manager.preStartCheck(); err == nil {
		t.Errorf("wanted non-nil error, got %v", err)
	}
}
