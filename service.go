package rxd

import (
	"context"
	"time"

	"github.com/ambitiousfew/rxd/config"
	"github.com/ambitiousfew/rxd/intracom"
	"github.com/ambitiousfew/rxd/pkg/rpc"
)

type ServiceRunner interface {
	Init(ServiceContext) error
	Idle(ServiceContext) error
	Run(ServiceContext) error
	Stop(ServiceContext) error
}

// Service is a struct that contains the Name of the service, the ServiceRunner and the ServiceHandler.
// This struct is what the caller uses to add a new service to the daemon.
// The daemon performs checks and translates this struct into a Service struct before starting it.
type Service struct {
	Name    string
	Runner  ServiceRunner
	Loader  config.LoaderFn
	Manager ServiceManager
}

// DaemonService is a struct that contains the Name of the service, the ServiceRunner
// this struct is what is passed into a Handler for the  handler to decide how to
// interact with the service using the ServiceRunner.
type ManagedService struct {
	Name            string
	Runner          ServiceRunner
	CommandC        <-chan rpc.CommandSignal
	ServiceLoaderFn config.LoaderFn
}

type DaemonState struct {
	configLoader config.Loader
	configC      <-chan int64
	logC         chan<- DaemonLog
	ic           *intracom.Intracom
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
			ConfigPolicy: ConfigPolicyStopFirst,
		},
		Loader: noopConfigLoadFn,
	}

	for _, opt := range opts {
		opt(&ds)
	}

	return ds
}

func noopConfigLoadFn(ctx context.Context, fields map[string]any) error {
	return nil
}
