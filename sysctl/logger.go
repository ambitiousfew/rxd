package sysctl

import "github.com/ambitiousfew/rxd/log"

var _ log.Logger = noopLogger{}

type noopLogger struct{}

func (l noopLogger) Log(level log.Level, message string, fields ...log.Field) {}
func (l noopLogger) SetLevel(level log.Level)                                 {}
