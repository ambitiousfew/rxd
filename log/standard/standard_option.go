package standard

import "github.com/ambitiousfew/rxd/log"

type StandardOption func(l *standardLogger)

func WithMessageFormat(format string) StandardOption {
	return func(l *standardLogger) {
		l.msgfmt = format
	}
}

func WithTimeFormat(format string) StandardOption {
	return func(l *standardLogger) {
		l.timefmt = format
	}
}

func WithLogLevel(level log.Level) StandardOption {
	return func(l *standardLogger) {
		l.level = level
	}
}
