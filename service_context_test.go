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
