package rxd

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestDaemonAddService(t *testing.T) {
	serviceName := "valid-service"

	vs := &validService{}
	validSvc := NewService(serviceName, vs)

	d := NewDaemon(DaemonConfig{
		Name:    "test-daemon",
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	})

	if len(d.services) != 0 {
		t.Errorf("daemon should not yet have any services")
	}

	err := d.AddService(validSvc)
	if err != nil {
		t.Errorf("error adding service: %s", err)
	}

	if len(d.services) != 1 {
		t.Errorf("daemon AddService did not correctly add new service")
	}

	_, found := d.services[serviceName]
	if !found {
		t.Errorf("could not find the service '%s' in the daemon services map", serviceName)
	}
}

func TestDaemonStartWithNoServices(t *testing.T) {
	d := NewDaemon(DaemonConfig{
		Name:    "test-daemon",
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	})

	if len(d.services) != 0 {
		t.Errorf("daemon should not yet have any services")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := d.Start(ctx)
	if err == nil {
		t.Errorf("daemon should not start without any services")
	}

}

func TestDaemonStartSingleService(t *testing.T) {
	serviceName := "valid-service"

	vs := &validService{}
	validSvc := NewService(serviceName, vs)

	d := NewDaemon(DaemonConfig{
		Name:    "test-daemon",
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := d.AddService(validSvc)
	if err != nil {
		t.Errorf("error adding service: %s", err)
	}

	// valid service has no blocking code so it will run all lifecycle stages and exit.
	err = d.Start(ctx)
	if err != nil {
		t.Errorf("expected: nil, got: %s", err)
	}

}

var _ Servicer = &validService{}

type validService struct{}

func (v *validService) Init(sc ServiceContext) ServiceResponse {
	return NewResponse(nil, Idle)
}

func (v *validService) Idle(sc ServiceContext) ServiceResponse {
	return NewResponse(nil, Run)
}

func (v *validService) Run(sc ServiceContext) ServiceResponse {
	return NewResponse(nil, Stop)
}

func (v *validService) Stop(sc ServiceContext) ServiceResponse {
	return NewResponse(nil, Exit)
}
