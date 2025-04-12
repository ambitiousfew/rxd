package log

import (
	"sync"
)

type logger struct {
	handler Handler
	fields  []Field
	level   *Level
	mu      *sync.RWMutex
}

func NewLogger(level Level, handler Handler) Logger {
	var lvl Level = level
	return &logger{
		handler: handler,
		fields:  []Field{},
		// since all child loggers will share the same level, we need to pass a pointer to the level
		level: &lvl,
		mu:    &sync.RWMutex{},
	}
}

func (l *logger) Log(level Level, message string, fields ...Field) {
	l.mu.RLock()
	ignore := *l.level < level
	l.mu.RUnlock()
	if ignore {
		// if the logger level is less than level passed, we don't log
		return
	}

	l.handler.Handle(level, message, fields)
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
	var lvl Level = level
	l.mu.Lock()
	l.level = &lvl
	l.mu.Unlock()
}
