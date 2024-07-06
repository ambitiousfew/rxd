package rxd

type ServiceOption func(*Service)

func UsingHandler(handler ServiceHandler) ServiceOption {
	return func(s *Service) {
		s.Handler = handler
	}
}
