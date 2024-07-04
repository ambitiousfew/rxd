package rxd

const (
	StateInit State = iota
	StateIdle
	StateRun
	StateStop
	StateExit
)

type State uint8

func (s State) String() string {
	switch s {
	case StateInit:
		return "Init"
	case StateIdle:
		return "Idle"
	case StateRun:
		return "Run"
	case StateStop:
		return "Stop"
	case StateExit:
		return "Exit"
	default:
		return "Unknown"
	}
}
