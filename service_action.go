package rxd

const (
	Entered ServiceAction = iota
	Exited
	Changed
	// Deprecated: In favor of using Entered
	Entering
	// Deprecated: In favor of using Exited
	Exiting
	// Deprecated: In favor of using Changed
	Changing
)

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
