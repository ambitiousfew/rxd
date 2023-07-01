package rxd

import (
	"testing"
	"time"
)

func TestAllServicesEnterStateHelperSyncPublish(t *testing.T) {
	testMock := &MockService{}
	testSvc := NewService("test-mock", testMock, NewServiceOpts())

	manager := newManager([]*ServiceContext{testSvc})
	manager.setLogger(&testLogger{})

	// // start manager state watcher, signal stop when test case ends.
	stopWatcherC := make(chan struct{})
	defer close(stopWatcherC)

	go func() {
		manager.serviceStateWatcher(stopWatcherC)
	}()

	// // ensure the service shares the same instance of intracom
	testSvc.setIntracom(manager.intracom)

	// Create interest in "test-mock" service state enter run
	statesC, cancelStatesC := AllServicesEnteredState(testSvc, RunState, "test-mock")
	defer cancelStatesC()
	// Simulate state transition to Init
	manager.updateServiceState(testSvc.Name, InitState)
	select {
	case states := <-statesC:
		if state := states[testSvc.Name]; state != InitState {
			t.Errorf("wanted %s, got %s", InitState, state)
		}
	default:
		// we are looking for entering of RunState not IdleState
	}
	// Simulate state transition to Idle
	manager.updateServiceState(testSvc.Name, IdleState)
	select {
	case states := <-statesC:
		if state := states[testSvc.Name]; state != IdleState {
			t.Errorf("wanted %s, got %s", IdleState, state)
		}
	default:
		// we are looking for entering of IdleState not InitState or RunState
	}
	// Simulate state transition to Run
	manager.updateServiceState(testSvc.Name, RunState)
	select {
	case states := <-statesC:
		if state := states[testSvc.Name]; state != RunState {
			t.Errorf("wanted %s, got %s", RunState, state)
		}
	default:
		// we are looking for entering of RunState not InitState or IdleState
	}
}

func TestAllServicesEnterStateHelperAsyncPublish(t *testing.T) {
	testMock := &MockService{}
	testSvc := NewService("test-mock", testMock, NewServiceOpts())
	manager := newManager([]*ServiceContext{testSvc})

	manager.setLogger(&testLogger{})

	// // ensure the service shares the same instance of intracom
	testSvc.setIntracom(manager.intracom)

	stopWatcherC := make(chan struct{})
	// ensure we kill state watcher when test finishes.
	defer close(stopWatcherC)
	go manager.serviceStateWatcher(stopWatcherC)

	go func() {
		// Simulate state transition to Init
		time.Sleep(10 * time.Millisecond)
		manager.updateServiceState(testSvc.Name, InitState)
		// Simulate state transition to Idle
		time.Sleep(10 * time.Millisecond)
		manager.updateServiceState(testSvc.Name, IdleState)
		// Simulate state transition to Run
		time.Sleep(10 * time.Millisecond)
		manager.updateServiceState(testSvc.Name, RunState)
	}()

	// Create interest in "test-mock" service state enter run
	allUpC, allUpCancel := AllServicesEnteredState(testSvc, RunState, "test-mock")
	defer allUpCancel()

	timeout := time.NewTimer(100 * time.Millisecond)
	defer timeout.Stop()

	select {
	case <-timeout.C:
		t.Errorf("did not receive state change within the timeout")
	case states := <-allUpC:
		timeout.Reset(100 * time.Millisecond)
		currentState := states[testSvc.Name]
		if currentState != RunState {
			t.Errorf("want %s, got %s", RunState, currentState)
		}
	}
}

func TestAnyServicesExitStateHelperAsyncPublish(t *testing.T) {
	testMock := &MockService{}
	testSvc := NewService("test-mock", testMock, NewServiceOpts())
	manager := newManager([]*ServiceContext{testSvc})
	manager.setLogger(&testLogger{})

	// ensure the service shares the same instance of intracom
	testSvc.setIntracom(manager.intracom)

	// Create interest in "test-mock" service state enter run
	anyDownC, anyDownCancel := AnyServicesExitState(testSvc, RunState, "test-mock")
	defer anyDownCancel()

	stopWatcherC := make(chan struct{})
	// ensure we kill state watcher when test finishes.
	defer close(stopWatcherC)
	go manager.serviceStateWatcher(stopWatcherC)

	go func() {
		// Simulate state transition to Init
		time.Sleep(10 * time.Millisecond)
		manager.updateServiceState(testSvc.Name, InitState)
		// Simulate state transition to Idle
		time.Sleep(10 * time.Millisecond)
		manager.updateServiceState(testSvc.Name, IdleState)
		// Simulate state transition to Run
		time.Sleep(10 * time.Millisecond)
		manager.updateServiceState(testSvc.Name, RunState)
		time.Sleep(10 * time.Millisecond)
		manager.updateServiceState(testSvc.Name, StopState)
	}()

	timeout := time.NewTimer(100 * time.Millisecond)
	defer timeout.Stop()

	select {
	case <-timeout.C:
		t.Errorf("did not receive state change within the timeout")
	case states := <-anyDownC:
		timeout.Reset(100 * time.Millisecond)
		currentState := states[testSvc.Name]
		if currentState == RunState {
			t.Errorf("want %s, got %s", StopState, currentState)
		}
	}
}
