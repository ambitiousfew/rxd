package rxd

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
