package rxd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

func TestDaemon_StartAService(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	internalWriter := new(strings.Builder)
	svcWriter := new(strings.Builder)

	testInternallogger := log.NewLogger(log.LevelDebug, newTestLogger(internalWriter))
	testServicelogger := log.NewLogger(log.LevelDebug, newTestLogger(svcWriter))

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

func TestDaemon_PanicService(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	internalWriter := new(strings.Builder)
	svcWriter := new(strings.Builder)

	testInternallogger := log.NewLogger(log.LevelDebug, newTestLogger(internalWriter))
	testServicelogger := log.NewLogger(log.LevelDebug, newTestLogger(svcWriter))

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

	if !strings.Contains(svcWriter.String(), "intentional panic") {
		t.Fatalf("expected panic message in service logger")
	}

	if !strings.Contains(internalWriter.String(), "intentional panic") {
		t.Fatalf("expected panic message in internal logger")
	}

}
