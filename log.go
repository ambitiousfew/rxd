package rxd

// Level is a representation of the logging level to be used.
type Level string

var (
	// Debug to log debug and higher
	Debug Level = "debug"
	// Info to log info and higher
	Info Level = "info"
	// Error to log only error and higher
	Error Level = "error"
)

type LogMessage struct {
	Message string
	Level   Level
}

// NewLog creates a new instance of a LogMessage from a string and level.
func NewLog(message string, level Level) LogMessage {
	return LogMessage{Message: message, Level: level}
}
