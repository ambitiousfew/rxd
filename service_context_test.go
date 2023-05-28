package rxd

import (
	"context"
	"testing"
	"time"
)

func TestServiceContextShutdownSignal(t *testing.T) {
	vs := &validService{}
	opts := NewServiceOpts()
	validSvc := NewService("valid-service", vs, opts)

	ch := validSvc.ShutdownSignal()
	validSvc.cancelCtx()

	// we cancel the service context immediately so it should always run first.
	testCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	select {
	case <-ch:
		return
	case <-testCtx.Done():
		t.Errorf("ServiceContext ShutdownSignal did not properly trigger")
	}
}

func TestServiceContextShutdownSuccessful(t *testing.T) {
	vs := &validService{}
	opts := NewServiceOpts()
	validSvc := NewService("valid-service", vs, opts)
	// Ensure service has a logC so it wont hang on channel based logging.
	validSvc.logC = make(chan LogMessage, 5)

	validSvc.shutdown()
	want := int32(1)
	got := validSvc.shutdownCalled.Load()

	if got != want {
		t.Errorf("ServiceContext shutdown() did not shutdown properly, wanted %d for shutdownCalled got %d", want, got)
	}
}

func TestServiceContextNotifyStateNoDependents(t *testing.T) {
	vs := &validService{}
	opts := NewServiceOpts()
	validSvc := NewService("valid-service", vs, opts)

	stateC := validSvc.ChangeState()
	validSvc.notifyStateChange(IdleState)
	// we cancel the service context immediately so it should always run first.
	testCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	select {
	case <-testCtx.Done():
		return
	case <-stateC:
		// no dependents to inform means stateC will never have a message.
		t.Errorf("ServiceContext ChangeState() should not have received a state with no dependents")
	}
}

func TestServiceContextNotifyStateWithDependents(t *testing.T) {
	vsParent := &validService{}
	optsParent := NewServiceOpts()
	validSvcParent := NewService("valid-parent-service", vsParent, optsParent)

	vsChild := &validService{}
	optsChild := NewServiceOpts()
	validSvcChild := NewService("valid-child-service", vsChild, optsChild)

	// Ensure service has a logC so it wont hang on channel based logging.
	validSvcParent.logC = make(chan LogMessage, 5)

	want := IdleState

	err := validSvcParent.AddDependentService(validSvcChild, want)
	if err != nil {
		t.Errorf("ServiceContext %s AddDependentService had an error: %s", validSvcParent.name, err)
	}
	// we cancel the service context immediately so it should always run first.
	testCtx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	defer validSvcParent.cancelCtx()

	go func() {
		timer := time.NewTimer(50 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-testCtx.Done():
			return
		case <-timer.C:
			// notifyStateChange sends the state up the parent channel where a notifier worker receives it.
			// we are just ensuring when there are dependents, that state is broadcasted up the channel.
			validSvcParent.notifyStateChange(want)
			return
		}
	}()

	select {
	case <-testCtx.Done():
		t.Errorf("ServiceContext %s notifyChangeState did not complete in a reasonable amount of time", validSvcParent.name)
	case state := <-validSvcParent.stateC:
		if want != state {
			t.Errorf("ServiceContext %s did not receive correct state change. want: %s, got: %s", validSvcChild.name, want, state)
		}
	}
}

func TestServiceContextAddSelfAsDependent(t *testing.T) {
	vsParent := &validService{}
	optsParent := NewServiceOpts()
	validSvcParent := NewService("valid-parent-service", vsParent, optsParent)

	// Ensure service has a logC so it wont hang on channel based logging.
	validSvcParent.logC = make(chan LogMessage, 5)

	err := validSvcParent.AddDependentService(validSvcParent, IdleState)
	if err == nil {
		t.Errorf("ServiceContext %s AddDependentService should not be able to add itself as a dependent", validSvcParent.name)
	}

	if len(validSvcParent.dependents) > 0 {
		t.Errorf("ServiceContext %s AddDependentService should not have dependents when trying to add itself as one", validSvcParent.name)
	}
}

func TestServiceContextAddChildAsDependent(t *testing.T) {
	vsParent := &validService{}
	optsParent := NewServiceOpts()
	validSvcParent := NewService("valid-parent-service", vsParent, optsParent)

	vsChild := &validService{}
	optsChild := NewServiceOpts()
	validSvcChild := NewService("valid-child-service", vsChild, optsChild)

	// Ensure service has a logC so it wont hang on channel based logging.
	validSvcParent.logC = make(chan LogMessage, 5)

	err := validSvcParent.AddDependentService(validSvcChild, IdleState)
	if err != nil {
		t.Errorf("ServiceContext %s AddDependentService errored with: %s", validSvcParent.name, err)
	}

	if len(validSvcParent.dependents) < 1 {
		t.Errorf("ServiceContext %s AddDependentService should have dependents. want %d, got: %d", validSvcParent.name, 1, len(validSvcChild.dependents))
	}
}

func TestServiceContextAddDependentNoStates(t *testing.T) {
	vsParent := &validService{}
	optsParent := NewServiceOpts()
	validSvcParent := NewService("valid-parent-service", vsParent, optsParent)

	vsChild := &validService{}
	optsChild := NewServiceOpts()
	validSvcChild := NewService("valid-child-service", vsChild, optsChild)

	// Ensure service has a logC so it wont hang on channel based logging.
	validSvcParent.logC = make(chan LogMessage, 5)

	err := validSvcParent.AddDependentService(validSvcChild)
	if err == nil {
		t.Errorf("ServiceContext %s AddDependentService should not be able to add a child service with no interested states", validSvcParent.name)
	}
}

func TestServiceContextServiceLogHelper(t *testing.T) {
	vsParent := &validService{}
	optsParent := NewServiceOpts()
	validSvcParent := NewService("valid-parent-service", vsParent, optsParent)

	// // Ensure service has a logC so it wont hang on channel based logging.
	// validSvcParent.logC = make(chan LogMessage, 5)
	want := "valid-parent-service test"
	got := serviceLog(validSvcParent, "test")

	if want != got {
		t.Errorf("serviceLog helper did not return correct string: want %s, got %s", want, got)
	}
}
