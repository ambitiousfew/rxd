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
	defer l.mu.RUnlock()
	if ignore {
		// if the logger level is less than level passed, we don't log
		return
	}
	allFields := make([]Field, len(l.fields)+len(fields))
	copy(allFields, append(l.fields, fields...))

	l.handler.Handle(level, message, allFields)
}

func (l *logger) With(fields ...Field) Logger {
	l.mu.RLock()
	level := *l.level
	fieldsCopy := make([]Field, len(l.fields)+len(fields))
	copy(fieldsCopy, append(l.fields, fields...))
	l.mu.RUnlock()

	return &logger{
		level:   &level,
		fields:  fieldsCopy,
		handler: l.handler,
		mu:      new(sync.RWMutex),
	}
}

func (l *logger) SetLevel(level Level) {
	var lvl Level = level
	l.mu.Lock()
	l.level = &lvl
	l.mu.Unlock()
}
