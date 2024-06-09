package rxd

type MockService struct{}

func (ms *MockService) Init(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, StateIdle)
}

func (ms *MockService) Idle(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, StateRun)
}

func (ms *MockService) Run(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, StateStop)
}

func (ms *MockService) Stop(sc *ServiceContext) ServiceResponse {
	return NewResponse(nil, StateExit)
}

// func TestAllServicesEnterStateHelperSyncPublish(t *testing.T) {
// 	iStates := intracom.New[States]("test-mock-states")
// 	err := iStates.Start()
// 	if err != nil {
// 		t.Errorf("failed to start intracom: %v", err)
// 	}
// 	defer iStates.Close()

// 	publishStateC, unregister := iStates.Register(internalServiceStates)
// 	defer unregister()

// 	testMock := &MockService{}
// 	testSvc := NewService("test-mock", testMock, NewServiceOpts())
// 	testSvc.iStates = iStates // ensure the service shares the same instance of intracom

// 	// Create interest in "test-mock" service state entering run state.
// 	enterStateC, enterStateCancel := AllServicesEnterState(testSvc, RunState, "test-mock")
// 	defer enterStateCancel()

// 	timer := time.NewTimer(50 * time.Millisecond)
// 	defer timer.Stop()

// 	go func() {
// 		states := make(States)
// 		states[testSvc.Name] = RunState

// 		select {
// 		case <-timer.C:
// 			return
// 		case publishStateC <- states:
// 		}
// 	}()

// 	select {
// 	case <-enterStateC:
// 		return // success
// 	case <-timer.C:
// 		t.Errorf("did not receive state change within the timeout")
// 	}
// }
