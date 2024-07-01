package rxd

import "context"

type ServiceHandler interface {
	Handle(ctx context.Context, service DaemonService, errC chan<- error)
}

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

type State uint8

func (s State) String() string {
	switch s {
	case Init:
		return "Init"
	case Idle:
		return "Idle"
	case Run:
		return "Run"
	case Stop:
		return "Stop"
	case Exit:
		return "Exit"
	default:
		return "Unknown"
	}
}

type Service struct {
	Name      string
	Runner    ServiceRunner
	RunPolicy RunPolicy
}

func NewService(name string, runner ServiceRunner, opts ...ServiceOption) Service {
	service := Service{
		Name:      name,
		Runner:    runner,
		RunPolicy: PolicyContinue,
	}

	for _, opt := range opts {
		opt(&service)
	}

	return service
}
