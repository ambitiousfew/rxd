package rxd

import (
	"io"
	"log"
	"os"
)

// LogSeverity is a typed int that represents the severity of the logging levels.
type LogSeverity int

const (
	// LevelAll will show all severity levels
	LevelAll LogSeverity = 0
	// LevelDebug will show all severity levels at Debug and above: Debug, Info, Error
	LevelDebug LogSeverity = 1
	// LevelInfo will show all severity levels at Info and above: Info, Error
	LevelInfo LogSeverity = 2
	// LevelWarn  LogSeverity = 3

	// LevelError will show all severity levels at Error and above: Error
	LevelError LogSeverity = 4
	// LevelFatal LogSeverity = 5

	// LevelOff will not show any logs, logging will be off
	LevelOff LogSeverity = 6
)

type Logger struct {
	Debug *log.Logger
	Info  *log.Logger
	Error *log.Logger
}

// NewLogger returns an instance of Logger with Debug, Info and Error loggers.
// Each logger is configured with sane defaults.
// Debug output is set with env var "DEBUG"; defaults to io.Discard
func NewLogger(logSeverity LogSeverity) *Logger {
	var debugOut io.Writer = os.Stdout
	var infoOut io.Writer = os.Stdout
	var errorOut io.Writer = os.Stderr

	switch logSeverity {
	case LevelInfo:
		debugOut = io.Discard
	case LevelError:
		debugOut = io.Discard
		infoOut = io.Discard
	case LevelOff:
		debugOut = io.Discard
		infoOut = io.Discard
		errorOut = io.Discard
	default:
		break
	}

	return &Logger{
		// 2022/10/23 09:21:45 main.go:8: [DEBUG] This is a DEBUG
		Debug: log.New(debugOut, "[DEBUG] ", log.LstdFlags|log.Lmsgprefix|log.Lshortfile),
		// 2022/10/23 09:21:45 [INFO] This is an INFO
		Info: log.New(infoOut, "[INFO] ", log.LstdFlags|log.Lmsgprefix),
		// 2022/10/23 09:44:16 main.go:11: [ERROR] This is an ERROR
		Error: log.New(errorOut, "[ERROR] ", log.LstdFlags|log.Lshortfile|log.Lmsgprefix),
	}
}
