package rxrpc

const (
	Unknown  Command = iota
	SetLevel         // SetLevel is a command to set the log level of the daemon
	// Start
	// Stop
	// Restart
	// Reload
	// Status
	// List
)

type Command int

func (r Command) String() string {
	switch r {
	case SetLevel:
		return "SetLevel"
	// case Start:
	// 	return "Start"
	// case Stop:
	// 	return "Stop"
	// case Restart:
	// 	return "Restart"
	// case Reload:
	// 	return "Reload"
	// case Status:
	// 	return "Status"
	// case List:
	// 	return "List"
	default:
		return "Unknown"
	}
}
