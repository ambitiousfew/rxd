package rxd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

type testLogger struct {
	t *testing.T
}

func (t testLogger) Handle(level log.Level, message string, fields []log.Field) {
	var buf strings.Builder
	for _, f := range fields {
		buf.WriteString(" ")
		buf.WriteString(f.Key)
		buf.WriteString("=")
		buf.WriteString(f.Value)
	}

	t.t.Logf("[%s] %s %s", level, message, buf.String())
}

func TestDaemon_StartAService(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := log.NewLogger(log.LevelDebug, testLogger{t: t})

	d := NewDaemon("test-daemon", WithInternalLogger(logger))

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
