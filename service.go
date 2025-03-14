package rxd

import (
	"time"

	"github.com/ambitiousfew/rxd/config"
	"github.com/ambitiousfew/rxd/intracom"
	"github.com/ambitiousfew/rxd/pkg/rpc"
)

type ServiceInitializer interface {
	Init(ServiceContext) error
}

type ServiceIdler interface {
	Idle(ServiceContext) error
}

type ServiceRunner interface {
	Run(ServiceContext) error
}

type ServiceStopper interface {
	Stop(ServiceContext) error
}

type ServiceReloader interface {
	Reload(ServiceContext ServiceContext, fields map[string]any) error
}

// type ServiceRunner interface {
// 	Init(ServiceContext) error
// 	Idle(ServiceContext) error
// 	Run(ServiceContext) error
// 	Stop(ServiceContext) error
// }

// Service is a struct that contains the Name of the service, the ServiceRunner and the ServiceHandler.
// This struct is what the caller uses to add a new service to the daemon.
// The daemon performs checks and translates this struct into a Service struct before starting it.
type Service struct {
	Name    string
	Runner  ServiceRunner
	Manager ServiceManager
}

// DaemonService is a struct that contains the Name of the service, the ServiceRunner
// this struct is what is passed into a Handler for the  handler to decide how to
// interact with the service using the ServiceRunner.
type ManagedService struct {
	Name     string
	Runner   ServiceRunner
	CommandC <-chan rpc.CommandSignal
}

type DaemonState struct {
	configC <-chan int64
	logC    chan<- DaemonLog
	updateC chan<- StateUpdate
	loader  config.Loader
	ic      *intracom.Intracom
}

func (ds DaemonState) NotifyState(serviceName string, state State) {
	ds.updateC <- StateUpdate{Name: serviceName, State: state}
}

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
