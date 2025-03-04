package sysctl

import "github.com/ambitiousfew/rxd/log"

type DefaultOption func(*defaultAgent)

func WithLogger(logger log.Logger) DefaultOption {
	return func(s *defaultAgent) {
		s.logger = logger
	}
}
