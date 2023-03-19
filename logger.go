package rxd

import (
	"io"
	"log"
	"os"
)

type LogSeverity int

const (
	LevelAll   LogSeverity = 0
	LevelDebug LogSeverity = 1
	LevelInfo  LogSeverity = 2
	// LevelWarn = 3?
	LevelError LogSeverity = 4
	// LevelFatal = 5?
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
