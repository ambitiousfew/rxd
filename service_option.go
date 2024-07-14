package rxd

type ServiceOption func(*Service)

func WithHandler(handler ServiceHandler) ServiceOption {
	return func(s *Service) {
		s.Handler = handler
	}
}
