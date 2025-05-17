package rxd

import (
	"time"

	"github.com/ambitiousfew/rxd/v2/config"
	"github.com/ambitiousfew/rxd/v2/intracom"
	"github.com/ambitiousfew/rxd/v2/log"
	"github.com/ambitiousfew/rxd/v2/pkg/rpc"
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

type ServiceLoader interface {
	Load(ServiceContext, []byte) error
}

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
	configC <-chan int64 // configC used by service managers to receive sighup signals
	// logC    chan<- DaemonLog   // logC used by service context
	updateC        chan<- StateUpdate // channel to send state updates to the daemon
	loader         config.Loader      // config loader for the daemon
	ic             *intracom.Intracom // daemon intracom instance used by all services
	internalLogger log.Logger         // internal daemon logger
	serviceLogger  log.Logger         // service logger
}

func (ds DaemonState) NotifyState(serviceName string, state State) {
	ds.updateC <- StateUpdate{Name: serviceName, State: state}
}

// Logger returns the internal service logger for the daemon.
func (ds DaemonState) Logger() log.Logger {
	return ds.serviceLogger
}

func (ds DaemonState) LoadSignal() <-chan int64 {
	return ds.configC
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
			LogWarningPolicy: LogWarningPolicyBoth,
			WarnDuration:     0,
		},
	}

	for _, opt := range opts {
		opt(&ds)
	}

	return ds
}
