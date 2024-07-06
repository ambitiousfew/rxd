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
		return "init"
	case StateIdle:
		return "idle"
	case StateRun:
		return "run"
	case StateStop:
		return "stop"
	case StateExit:
		return "exit"
	default:
		return "unknown"
	}
}
