package rxd

import "time"

type ServiceOption func(*Service)

func WithHandler(handler ServiceHandler) ServiceOption {
	return func(s *Service) {
		s.Handler = handler
	}
}

func WithTransitionTimeout(transition StateTransition, timeout time.Duration) ServiceOption {
	return func(s *Service) {
		s.TransitionTimeouts[transition] = timeout
	}
}
