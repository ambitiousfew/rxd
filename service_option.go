package rxd

type ServiceOption func(*Service)

func WithManager(manager ServiceManager) ServiceOption {
	return func(s *Service) {
		s.Manager = manager
	}
}
