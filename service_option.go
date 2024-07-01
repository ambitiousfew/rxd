package rxd

type ServiceOption func(*Service)

func UsingRunPolicy(policy RunPolicy) ServiceOption {
	return func(s *Service) {
		s.RunPolicy = policy
	}
}
