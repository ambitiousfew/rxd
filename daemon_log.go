package rxd

import "github.com/ambitiousfew/rxd/log"

type DaemonLog struct {
	Name    string
	Level   log.Level
	Message string
}

func (l DaemonLog) String() string {
	return l.Message
}
