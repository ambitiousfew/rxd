package rxd

// ServiceOption is a functional option type for configuring a Service.
type ServiceOption func(*Service)

// WithManager allows overriding the default ServiceManager for a Service.
func WithManager(manager ServiceManager) ServiceOption {
	return func(s *Service) {
		s.Manager = manager
	}
}
