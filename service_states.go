package rxd

type stateUpdate struct {
	service string
	state   ServiceState
}

// ServiceState represents the possible states a service can be in.
type ServiceState int

func (s ServiceState) String() string {
	switch s {
	case unknown:
		return "unknown"
	case Init:
		return "init"
	case Idle:
		return "idle"
	case Run:
		return "run"
	case Stop:
		return "stop"
	case Exit:
		return "exit"
	default:
		return "unknown"
	}
}

const (
	// unexported to prevent user selecting states that aren't lifecycles.
	unknown ServiceState = iota
	Init
	Idle
	Run
	Stop
	Exit
)
