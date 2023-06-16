package rxd

import (
	"testing"
)

type testLogger struct{}

func (tl *testLogger) Debug(v ...any) {}
func (tl *testLogger) Info(v ...any)  {}
func (tl *testLogger) Warn(v ...any)  {}
func (tl *testLogger) Error(v ...any) {}

func (tl *testLogger) Debugf(format string, v ...any) {}
func (tl *testLogger) Infof(format string, v ...any)  {}
func (tl *testLogger) Warnf(format string, v ...any)  {}
func (tl *testLogger) Errorf(format string, v ...any) {}

func TestDaemonSetLogger(t *testing.T) {
	vs := &validService{}
	vsOpts := NewServiceOpts()
	validSvc := NewService("valid-service", vs, vsOpts)

	d := NewDaemon(validSvc)
	tLogger := &testLogger{}
	d.SetCustomLogger(tLogger)

	if d.logger != tLogger {
		t.Errorf("daemon SetLogger did not set the correct logging instance")
	}
}

func TestDaemonGetLogger(t *testing.T) {
	vs := &validService{}
	vsOpts := NewServiceOpts()
	validSvc := NewService("valid-service", vs, vsOpts)

	d := NewDaemon(validSvc)
	tLogger := &testLogger{}
	d.SetCustomLogger(tLogger)

	if d.Logger() != tLogger {
		t.Errorf("daemon Logger did not return the correct logging instance")
	}
}

func TestDaemonAddService(t *testing.T) {
	vs := &validService{}
	vsOpts := NewServiceOpts()
	validSvc := NewService("valid-service", vs, vsOpts)

	d := NewDaemon()

	if len(d.manager.services) != 0 {
		t.Errorf("daemon instance was not initialized with 0 services")
	}

	d.AddService(validSvc)
	if len(d.manager.services) != 1 {
		t.Errorf("daemon AddService did not correctly add new service")
	}
}

func TestDaemonSignalWatcherByStopC(t *testing.T) {
	d := NewDaemon()

	go d.signalWatcher()

	close(d.stopCh)

	select {
	case <-d.stopCh:
		return
	default:
		t.Errorf("daemon signalWatcher did not stop the correct way")
	}
}

func TestDaemonStartNoServices(t *testing.T) {
	d := NewDaemon()
	err := d.Start()

	if err != nil {
		t.Errorf("daemon Start had an error: %s", err)
	}

}

func TestDaemonStartSingleService(t *testing.T) {
	vs := &validService{}
	vsOpts := NewServiceOpts()
	validSvc := NewService("valid-service", vs, vsOpts)

	d := NewDaemon(validSvc)
	err := d.Start()

	if err != nil {
		t.Errorf("daemon Start had an error: %s", err)
	}

}
