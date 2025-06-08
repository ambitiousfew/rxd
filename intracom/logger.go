package intracom

import "github.com/ambitiousfew/rxd/log"

type noopLogger struct{}

func (l noopLogger) Log(_ log.Level, _ string, _ ...log.Field) {}

func (l noopLogger) SetLevel(_ log.Level) {}

func (l noopLogger) With(_ ...log.Field) log.Logger {
	return l
}
