package rxd

const (
	// Entering indicates the service is entering a state.
	Entering ServiceAction = iota // 0
	// Exited idicates the service has exited a state.
	Exited // 1
	// Changing indicates the service has changed its state.
	// This would trigger at least twice per state: exited previous state and then entered new state.
	Changing
	// NotIn used to include all states except a given state.
	// This would also trigger at least twice per state: exited previous state and then entered new state.
	// But it would not trigger for the state that is being excluded.
	NotIn // 6
)

// ServiceAction represents different actions that can be watched for in a service's lifecycle.
// It is used by the ServiceWatcher to determine how to filter what to watch when inspecting
// service states.
type ServiceAction uint8

func (s ServiceAction) String() string {
	switch s {
	case Changing:
		return "changing"
	case Entering:
		return "entering"
	case Exited:
		return "exited"
	case NotIn:
		return "not_in"
	default:
		return "unknown"
	}
}
