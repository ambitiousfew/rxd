package rxd

const (
	// InitState is in the ServiceResponse to inform manager to move us to the Init state (Initial Default).
	InitState State = iota
	// IdleState is in the ServiceResponse to inform manager to move us to the Idle state
	IdleState
	// RunState is in the ServiceResponse to inform manager to move us to the Run state
	RunState
	// StopState is in the ServiceResponse to inform manager to move us to the Stop state
	StopState
	// ExitState is in the ServiceResponse to inform manager to act as the final response type for Stop.
	ExitState
)

// State is used to determine the "next state" the service should enter
// when the current state has completed/errored returned. State should
// reflect different states that the interface can enter.
type State int

func (s State) String() string {
	switch s {
	case InitState:
		return "init"
	case IdleState:
		return "idle"
	case RunState:
		return "run"
	case StopState:
		return "stop"
	case ExitState:
		return "exit"
	default:
		return "unknown"
	}
}

type Service interface {
	Init(*ServiceContext) ServiceResponse
	Idle(*ServiceContext) ServiceResponse
	Run(*ServiceContext) ServiceResponse
	Stop(*ServiceContext) ServiceResponse
}
