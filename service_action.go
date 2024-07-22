package rxd

const (
	Entering ServiceAction = iota
	Exiting
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
	default:
		return "unknown"
	}
}
