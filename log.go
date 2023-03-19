package rxdaemon

type Level string

var (
	Debug Level = "debug"
	Info  Level = "info"
	Error Level = "error"
)

type LogMessage struct {
	Message string
	Level   Level
}

func NewLog(message string, level Level) LogMessage {
	return LogMessage{Message: message, Level: level}
}
