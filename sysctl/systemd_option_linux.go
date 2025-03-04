// build +linux
package sysctl

import (
	"os"
	"syscall"
	"time"
)

type SystemdAgentOption func(*systemdAgent)

func WithNotifyReady(notifyReady bool) SystemdAgentOption {
	return func(a *systemdAgent) {
		a.notifyReady = notifyReady
	}
}

func WithWatchdog(interval time.Duration) SystemdAgentOption {
	return func(a *systemdAgent) {
		a.enableWatchdog = true
		a.watchdogInterval = interval
	}
}

func WithSignals(signals ...os.Signal) SystemdAgentOption {
	return func(a *systemdAgent) {
		if len(signals) == 0 {
			signals = []os.Signal{os.Interrupt, syscall.SIGTERM}
		}

		a.signals = signals
	}
}
