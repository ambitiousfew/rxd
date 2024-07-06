package journald

import "github.com/ambitiousfew/rxd/log"

type Option func(l *journaldLogger)

func WithSeverityPrefix(enabled bool) Option {
	return func(l *journaldLogger) {
		l.severityPrefix = enabled
	}
}

func WithLogLevel(level log.Level) Option {
	return func(l *journaldLogger) {
		l.level = level
	}
}
