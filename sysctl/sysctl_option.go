package sysctl

import (
	"os"

	"github.com/ambitiousfew/rxd/log"
)

type DefaultOption func(*defaultAgent)

func WithOSSignals(signals ...os.Signal) DefaultOption {
	return func(s *defaultAgent) {
		s.signals = signals
	}
}

func WithCustomLogger(logger log.Logger) DefaultOption {
	return func(s *defaultAgent) {
		s.logger = logger
	}
}
