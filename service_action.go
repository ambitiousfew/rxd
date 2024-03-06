package rxd

const (
	Entering ServiceAction = iota
	Exiting
)

type ServiceAction int

func (s ServiceAction) String() string {
	switch s {
	case Entering:
		return "entering"
	case Exiting:
		return "exiting"
	default:
		return "unknown"
	}
}
