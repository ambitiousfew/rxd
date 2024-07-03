package rxd

type ServiceOption func(*DaemonService)

func UsingHandler(handler ServiceHandler) ServiceOption {
	return func(s *DaemonService) {
		s.Handler = handler
	}
}
