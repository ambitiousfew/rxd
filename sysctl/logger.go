package sysctl

import "github.com/ambitiousfew/rxd/v2/log"

var _ log.Logger = noopLogger{}

type noopLogger struct{}

func (l noopLogger) Log(level log.Level, message string, fields ...log.Field) {}
func (l noopLogger) SetLevel(level log.Level)                                 {}
func (l noopLogger) With(fields ...log.Field) log.Logger                      { return l }
