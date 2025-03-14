package intracom

import "github.com/ambitiousfew/rxd/log"

type noopLogger struct{}

func (l noopLogger) Log(level log.Level, msg string, fields ...log.Field) {}
func (l noopLogger) SetLevel(level log.Level)                             {}
func (l noopLogger) With(fields ...log.Field) log.Logger                  { return l }
