package sysctl

import (
	"os"

	"github.com/ambitiousfew/rxd/log"
)

type DefaultOption func(*defaultAgent)

func WithSignals(signals ...os.Signal) DefaultOption {
	return func(s *defaultAgent) {
		s.signals = signals
	}
}

func WithLogger(logger log.Logger) DefaultOption {
	return func(s *defaultAgent) {
		s.logger = logger
	}
}
