package log

import (
	"sync"
)

// NewLogger creates a new Logger with the specified level and handler.
// The logger will use the provided handler to handle log messages.
// The level determines the minimum log level that will be logged.
// If the level is lower than the log level of the logger, the message will not be logged.
// The logger is safe for concurrent use, allowing multiple goroutines to log messages simultaneously.
func NewLogger(level Level, handler Handler) Logger {
	var lvl = level
	return &logger{
		handler: handler,
		fields:  []Field{},
		// since all child loggers will share the same level, we need to pass a pointer to the level
		level: &lvl,
		mu:    &sync.RWMutex{},
	}
}

type logger struct {
	handler Handler
	fields  []Field
	level   *Level
	mu      *sync.RWMutex
}

func (l *logger) Log(level Level, message string, fields ...Field) {
	l.mu.RLock()
	ignore := *l.level < level
	l.mu.RUnlock()
	if ignore {
		// if the logger level is less than level passed, we don't log
		return
	}

	// Combine the logger's fields with the provided log fields
	allFields := make([]Field, len(l.fields)+len(fields))
	copy(allFields, l.fields)
	copy(allFields[len(l.fields):], fields)

	l.handler.Handle(level, message, allFields)
}

func (l *logger) With(fields ...Field) Logger {
	return &logger{
		level:   l.level,
		fields:  append(l.fields, fields...),
		handler: l.handler,
		mu:      l.mu,
	}
}

func (l *logger) SetLevel(level Level) {
	var lvl = level
	l.mu.Lock()
	l.level = &lvl
	l.mu.Unlock()
}
