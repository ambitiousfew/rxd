package rxd

import "github.com/ambitiousfew/rxd/log"

type ServiceLogger interface {
	Log(level log.Level, message string, extra ...log.Field)
}

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
	Handler ServiceHandler
}

// DaemonService is a struct that contains the Name of the service, the ServiceRunner
// this struct is what is passed into a Handler for the  handler to decide how to
// interact with the service using the ServiceRunner.
type daemonService struct {
	name   string
	runner ServiceRunner
}

func NewService(name string, runner ServiceRunner, opts ...ServiceOption) Service {
	ds := Service{
		Name:    name,
		Runner:  runner,
		Handler: DefaultHandler,
	}

	for _, opt := range opts {
		opt(&ds)
	}

	return ds
}
