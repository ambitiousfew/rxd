package rxd

type ServiceRunner interface {
	Init(ServiceContext) error
	Idle(ServiceContext) error
	Run(ServiceContext) error
	Stop(ServiceContext) error
}

const (
	Init State = iota
	Idle
	Run
	Stop
	Exit
)

// Service is a struct that contains the Name of the service, the ServiceRunner
// this struct is what is passed into a Handler for the  handler to decide how to
// interact with the service using the ServiceRunner.
type Service struct {
	Name   string
	Runner ServiceRunner
}

// DaemonService is a struct that contains the Name of the service, the ServiceRunner and the ServiceHandler.
// This struct is what the caller uses to add a new service to the daemon.
// The daemon performs checks and translates this struct into a Service struct before starting it.
type DaemonService struct {
	Name    string
	Runner  ServiceRunner
	Handler ServiceHandler
}

func NewService(name string, runner ServiceRunner, opts ...ServiceOption) DaemonService {
	ds := DaemonService{
		Name:    name,
		Runner:  runner,
		Handler: ServiceHandlerContinous{},
	}

	for _, opt := range opts {
		opt(&ds)
	}

	return ds
}
