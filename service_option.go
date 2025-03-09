package rxd

import "github.com/ambitiousfew/rxd/config"

type ServiceOption func(*Service)

func WithManager(manager ServiceManager) ServiceOption {
	return func(s *Service) {
		s.Manager = manager
	}
}

func WithConfigLoader(loader config.LoaderFn) ServiceOption {
	return func(s *Service) {
		if s.Loader == nil {
			s.Loader = noopConfigLoadFn
		}
		s.Loader = loader
	}
}
