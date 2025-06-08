package rpc

const (
	// Unknown Command is used when the command is not recognized.
	Unknown Command = iota
	// SetLevel Command is used to set the log level of a service.
	SetLevel
)

// Command represents the type of command that can be sent to a service.
// It is used to identify the action that should be performed by the service.
type Command uint8

// String returns a string representation of the Command.
func (c Command) String() string {
	switch c {
	case SetLevel:
		return "SetLevel"
	default:
		return "Unknown"
	}
}
