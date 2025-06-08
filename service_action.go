package rxd

//revive:disable:exported
const (
	// Entered indicates that the service has entered a state.
	Entered ServiceAction = iota // 0
	// Exited indicates that the service has exited a state.
	Exited // 1
	// Changed indicates that the service has changed its state.
	Changed // 2
	// Entering is used to indicate that the service is entering a specific state.
	Entering // 3
	// Deprecated: In favor of using Exited.
	Exiting
	// Deprecated: In favor of using Changed.
	Changing
	// NotIn is used as in inverse action to indicate that the service is not in a specific state.
	NotIn // 6
)

//revive:enable:exported

// ServiceAction represents different actions that can be watched for in a service's lifecycle.
// It is used by the ServiceWatcher to determine how to filter what to watch when inspecting
// service states.
type ServiceAction uint8

func (s ServiceAction) String() string {
	switch s {
	case Entering:
		return "entering"
	case Exiting:
		return "exiting"
	case Changing:
		return "changing"
	case Entered:
		return "entered"
	case Exited:
		return "exited"
	case Changed:
		return "changed"
	default:
		return "unknown"
	}
}
