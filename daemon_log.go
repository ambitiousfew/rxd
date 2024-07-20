package rxd

import (
	"github.com/ambitiousfew/rxd/log"
)

type DaemonLog struct {
	Name    string
	Level   log.Level
	Message string
	Fields  []log.Field
}

func (l DaemonLog) String() string {
	return l.Message
}

type noopLogHandler struct{}

func (h noopLogHandler) Handle(level log.Level, message string, fields []log.Field) {
	// do something with the log
	return
}
