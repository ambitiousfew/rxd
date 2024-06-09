package rxd

// func TestServiceShutdownContext(t *testing.T) {
// 	// we cancel the service context immediately so it should always run first.
// 	testCtx, cancel := context.WithCancel(context.Background())

// 	vs := &validService{}
// 	opts := NewServiceOpts(UsingContext(testCtx))
// 	validSvc := NewService("valid-service", vs, opts)

// 	cancel()

// 	timer := time.NewTimer(50 * time.Millisecond)
// 	defer timer.Stop()
// 	select {
// 	case <-validSvc.ShutdownCtx.Done():
// 		return
// 	case <-timer.C:
// 		t.Errorf("ServiceContext ShutdownSignal did not properly trigger")
// 	}
// }

// func TestServiceContextShutdownSuccessful(t *testing.T) {
// 	vs := &validService{}
// 	opts := NewServiceOpts()
// 	validSvc := NewService("valid-service", vs, opts)
// 	// Ensure service has a logC so it wont hang on channel based logging.

// 	validSvc.shutdown()
// 	want := int32(1)
// 	got := validSvc.shutdownCalled.Load()

// 	if got != want {
// 		t.Errorf("ServiceContext shutdown() did not shutdown properly, wanted %d for shutdownCalled got %d", want, got)
// 	}
// }
