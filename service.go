package rxd

// State is used to determine the "next state" the service should enter
// when the current state has completed/errored returned. State should
// reflect different states that the interface can enter.
type State string

const (
	// InitState is in the ServiceResponse to inform manager to move us to the Init state
	InitState State = "init"
	// IdleState is in the ServiceResponse to inform manager to move us to the Init state
	IdleState State = "idle"
	// RunState is in the ServiceResponse to inform manager to move us to the Init state
	RunState State = "run"
	// StopState is in the ServiceResponse to inform manager to move us to the Init state
	StopState State = "stop"
	// NoopState is in the ServiceResponse to inform manager to move us to the Init state
	NoopState State = "noop"
)

// Service is the service interface that all services should implement for Manager to be able to interact properly with it.
type Service interface {
	// Name is just a user-friendly or logging-friendly representation of the service name
	Name() string
	// Config should return a pointer reference to a ServiceConfig stored within the service
	Config() *ServiceConfig
	// Init is the initializaiton service state: any prep your service needs before running
	Init() ServiceResponse
	// Idle is the idling service state: Status checks, Test connection, etc before running or to fall back to when run fails.
	Idle() ServiceResponse
	// Run is the running service state: The heart of your service code is here, primary functionality should be done in this state.
	Run() ServiceResponse
	// Stop is the stop service state: Here you would do any cleanup of your service as your service is about to stop running.
	Stop() ServiceResponse
}
