package rxd

import (
	"context"
	"os"
	"syscall"
	"testing"
)

func TestDaemonAddService(t *testing.T) {
	serviceName := "valid-service"

	vs := &validService{}
	vsOpts := NewServiceOpts() // use defaults
	validSvc := NewService(serviceName, vs, vsOpts)

	d := NewDaemon(DaemonConfig{
		Name:    "test-daemon",
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	})

	if d.total != 0 {
		t.Errorf("daemon should not yet have any services")
	}

	err := d.AddService(validSvc)
	if err != nil {
		t.Errorf("error adding service: %s", err)
	}

	if d.total != 1 {
		t.Errorf("daemon AddService did not correctly add new service")
	}

	_, found := d.services.Load(serviceName)
	if !found {
		t.Errorf("could not find the service '%s' in the daemon services map", serviceName)
	}
}

func TestDaemonStartWithNoServices(t *testing.T) {
	d := NewDaemon(DaemonConfig{
		Name:    "test-daemon",
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	})

	if d.total != 0 {
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
	vsOpts := NewServiceOpts()
	validSvc := NewService(serviceName, vs, vsOpts)

	d := NewDaemon(DaemonConfig{
		Name:    "test-daemon",
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	})

	ctx, cancel := context.WithCancel(context.Background())
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
