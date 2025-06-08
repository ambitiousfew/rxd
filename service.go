package rxd

import "time"

// ServiceRunner is an interface that defines the lifecycle methods for a service.
type ServiceRunner interface {
	Init(ServiceContext) error
	Idle(ServiceContext) error
	Run(ServiceContext) error
	Stop(ServiceContext) error
}

// Service acts as a DTO that carries the Name of the service along
// with the ServiceRunner implementation and an optional ServiceManager.
// This is normally passed to the Daemon via AddService or AddServices methods.
type Service struct {
	Name    string
	Runner  ServiceRunner
	Manager ServiceManager
}

// DaemonService is a DTO passed to a service manager containing the Name of the service
// along with the the ServiceRunner implementation for the manager to use.
// Every service is wrapped in a Service Manager to handle the lifecycle of the service.
type DaemonService struct {
	Name   string
	Runner ServiceRunner
}

// NewService creates a new Service DTO with the provided name, runner and options.
// This DTO is used to register a service with the Daemon via AddService or AddServices methods.
func NewService(name string, runner ServiceRunner, opts ...ServiceOption) Service {
	ds := Service{
		Name:   name,
		Runner: runner,
		Manager: RunContinuousManager{
			// the first time we init the service we will short delay by 10 nanoseconds.
			StartupDelay: 10 * time.Nanosecond,
			// default state timeouts for all other states if not set specifically in state timeouts.
			DefaultDelay: 10 * time.Nanosecond,
			StateTimeouts: ManagerStateTimeouts{
				// re-inits from stop to init will delay by 5 seconds.
				StateInit: 5 * time.Second,
			},
		},
	}

	for _, opt := range opts {
		opt(&ds)
	}

	return ds
}
