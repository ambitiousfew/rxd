package rxd

import (
	"time"
)

type ServiceRunner interface {
	Init(ServiceContext) (State, error)
	Idle(ServiceContext) (State, error)
	Run(ServiceContext) (State, error)
	Stop(ServiceContext) (State, error)
}

// Service is a struct that contains the Name of the service, the ServiceRunner and the ServiceHandler.
// This struct is what the caller uses to add a new service to the daemon.
// The daemon performs checks and translates this struct into a Service struct before starting it.
type Service struct {
	Name    string
	Runner  ServiceRunner
	Handler ServiceHandler
}

// DaemonService is a struct that contains the Name of the service, the ServiceRunner
// this struct is what is passed into a Handler for the  handler to decide how to
// interact with the service using the ServiceRunner.
type DaemonService struct {
	Name   string
	Runner ServiceRunner
}

func NewService(name string, runner ServiceRunner, opts ...ServiceOption) Service {
	ds := Service{
		Name:   name,
		Runner: runner,
		Handler: DefaultHandler{
			StartupDelay: 10 * time.Nanosecond, // applies to first time init.
			StateTimeouts: HandlerStateTimeouts{
				StateInit: 5 * time.Second, // applies to all re-inits after the first time.
			},
		},
	}

	for _, opt := range opts {
		opt(&ds)
	}

	return ds
}
