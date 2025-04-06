package rxd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

func TestDaemon_StartService(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	internalTestLogger := newTestLogger()
	svcTestLogger := newTestLogger()

	testInternallogger := log.NewLogger(log.LevelDebug, internalTestLogger)
	testServicelogger := log.NewLogger(log.LevelDebug, svcTestLogger)

	d := NewDaemon("test-daemon", WithInternalLogger(testInternallogger), WithServiceLogger(testServicelogger))

	s1 := NewService("test-service-1", newMockService(100*time.Millisecond))

	err := d.AddServices(s1)
	if err != nil {
		t.Fatalf("error adding services: %s", err)
	}

	err = d.Start(ctx)
	if err != nil {
		t.Fatalf("error starting daemon: %s", err)
	}

	if !strings.Contains(svcTestLogger.Output(), "mockService.Init") {
		t.Fatalf("expected service init message in internal logger")
	}

	if !strings.Contains(svcTestLogger.Output(), "mockService.Idle") {
		t.Fatalf("expected service idle message in service logger")
	}

	if !strings.Contains(svcTestLogger.Output(), "mockService.Run") {
		t.Fatalf("expected service run message in service logger")
	}

	if !strings.Contains(svcTestLogger.Output(), "mockService.Stop") {
		t.Fatalf("expected service stop message in service logger")
	}
}

func TestDaemon_AddService(t *testing.T) {
	d := NewDaemon("test-daemon")

	s := NewService("test-service", newMockService(100*time.Millisecond))

	err := d.AddService(s)
	if err != nil {
		t.Fatalf("error adding service: %s", err)
	}
}

func TestDaemon_AddServices(t *testing.T) {
	d := NewDaemon("test-daemon")

	s1 := NewService("test-service-1", newMockService(100*time.Millisecond))
	s2 := NewService("test-service-2", newMockService(100*time.Millisecond))

	err := d.AddServices(s1, s2)
	if err != nil {
		t.Fatalf("error adding services: %s", err)
	}
}

func TestDaemon_AddUnnamedService(t *testing.T) {
	d := NewDaemon("test-daemon")

	// unnamed service causes an error during AddService
	s := NewService("", newMockService(100*time.Millisecond))
	err := d.AddService(s)
	if err == nil {
		t.Fatalf("expected error adding unnamed service, got: %v", err)
	}
}

func TestDaemon_PanicService(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	internalTestLogger := newTestLogger()
	svcTestLogger := newTestLogger()
	testInternallogger := log.NewLogger(log.LevelDebug, internalTestLogger)
	testServicelogger := log.NewLogger(log.LevelDebug, svcTestLogger)

	d := NewDaemon("test-daemon", WithInternalLogger(testInternallogger), WithServiceLogger(testServicelogger))

	s := NewService("test-service", newMockPanicService(100*time.Millisecond))

	err := d.AddService(s)
	if err != nil {
		t.Fatalf("error adding service: %s", err)
	}

	err = d.Start(ctx)
	if err != nil {
		t.Fatalf("expected no error starting daemon: %s", err)
	}

	if !strings.Contains(internalTestLogger.Output(), "intentional panic") {
		t.Fatalf("expected panic message in service logger")
	}

	if !strings.Contains(svcTestLogger.Output(), "intentional panic") {
		t.Fatalf("expected panic message in service logger")
	}
}

func TestDaemon_Options(t *testing.T) {
	opts := []DaemonOption{
		WithConfigLoader(nil),
		WithInternalLogger(nil),
		WithServiceLogger(nil),
		WithSystemAgent(nil),
		WithLogWorkerCount(0),
	}

	d := NewDaemon("test-daemon", opts...)
	testDaemon, ok := d.(*daemon)
	if !ok {
		t.Fatalf("expected daemon type")
	}

	if testDaemon.agent == nil {
		t.Fatalf("expected non-nil agent")
	}

	if testDaemon.configuration == nil {
		t.Fatalf("expected non-nil configuration")
	}

	if testDaemon.serviceLogger == nil {
		t.Fatalf("expected non-nil service logger")
	}

	if testDaemon.internalLogger == nil {
		t.Fatalf("expected non-nil internal logger")
	}

	if testDaemon.logWorkerCount == 0 {
		t.Fatalf("expected non-zero log worker count")
	}
}
