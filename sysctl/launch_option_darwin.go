//go:build darwin

package sysctl

import (
	"os"
	"syscall"
)

type LaunchdOption func(*launchdAgent)

func WithSignals(signals ...os.Signal) LaunchdOption {
	return func(a *launchdAgent) {
		if len(signals) == 0 {
			signals = []os.Signal{os.Interrupt, syscall.SIGTERM}
		}

		a.signals = signals
	}
}
