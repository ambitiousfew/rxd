package rxd

import (
	"context"
	"testing"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

func TestDaemon_StartAService(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	d := NewDaemon("test-daemon", noopLogger{})

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
	d := NewDaemon("test-daemon", noopLogger{})

	s := NewService("test-service", newMockService(100*time.Millisecond))

	err := d.AddService(s)
	if err != nil {
		t.Fatalf("error adding service: %s", err)
	}
}

func TestDaemon_AddServices(t *testing.T) {
	d := NewDaemon("test-daemon", noopLogger{})

	s1 := NewService("test-service-1", newMockService(100*time.Millisecond))
	s2 := NewService("test-service-2", newMockService(100*time.Millisecond))

	err := d.AddServices(s1, s2)
	if err != nil {
		t.Fatalf("error adding services: %s", err)
	}
}

type noopLogger struct{}

func (n noopLogger) Log(level log.Level, message string, fields ...log.Field) {}
func (n noopLogger) SetLevel(level log.Level)                                 {}
