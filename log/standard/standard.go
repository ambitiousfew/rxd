package standard

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

type standardLogger struct {
	timefmt string
	msgfmt  string
	level   log.Level
	fields  []log.Field
	stdout  io.Writer
	stderr  io.Writer
	lvlMu   sync.RWMutex // mutex for level and fields
	outMu   sync.RWMutex // mutex for stdout writer
	errMu   sync.RWMutex // mutex for stderr writer
}

// NewDefaultLogger returns a new logger with default settings
// The default logger settings are not configurable with exception of log level given.
// The defaults already set are:
// - no logger name
// - no log group
// - time format is RFC3339
// - log level is the level passed in
// - stdout is os.Stdout
// - stderr is os.Stderr
func NewDefaultLogger(level log.Level) log.Logger {
	return NewLogger(
		os.Stdout,
		os.Stderr,
		WithLogLevel(level),
		WithMessageFormat("{time} [{level}] {message}"),
		WithTimeFormat(time.RFC3339),
	)
}

// NewLogger creates a new logger allowing for custom stdout and stderr writers
// The logger will use the default settings:
// - no logger name, can override with: UsingLogName(<name>)
// - no log group, can override with: With(<group name>)
// - time format is RFC3339, can override with: UsingLogTimeFormat(<time format>)
// - log level is INFO, can override with: UsingLogLevel(<log level>)
func NewLogger(stdout, stderr io.Writer, opts ...StandardOption) log.Logger {
	l := &standardLogger{
		timefmt: time.RFC3339,
		msgfmt:  "{time} [{level}] {message}",
		level:   log.LevelInfo,
		fields:  []log.Field{},
		stdout:  stdout,
		stderr:  stderr,
		lvlMu:   sync.RWMutex{},
		errMu:   sync.RWMutex{},
		outMu:   sync.RWMutex{},
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// SetLevel sets the log level for the logger
// because concurrent writes can be taking place at the same time,
// we need to ensure the writers are locked trying to make changes
// to the log level.
func (l *standardLogger) SetLevel(level log.Level) {
	l.lvlMu.Lock()
	l.level = level
	l.lvlMu.Unlock()
}

// Log handles the logging of messages to the logger
func (l *standardLogger) Log(level log.Level, msg string, fields ...log.Field) {
	// if the logger level is less than level passed, we don't log
	l.lvlMu.RLock()
	if l.level < level {
		l.lvlMu.RUnlock()
		return
	}
	l.lvlMu.RUnlock()

	// replace the main fields first.
	message := strings.Replace(l.msgfmt, "{time}", time.Now().Format(l.timefmt), 1)
	message = strings.Replace(message, "{level}", level.String(), 1)
	message = strings.Replace(message, "{message}", msg, 1)

	var b strings.Builder
	_, err := b.WriteString(message)
	if err != nil {
		l.logErr("error logging: " + err.Error())
		return
	}

	allFields := append(l.fields, fields...)

	for _, field := range allFields {
		b.WriteString(" " + field.Key + "=" + field.Value)
	}

	switch level {
	case log.LevelError:
		l.logErr(b.String())
	default:
		l.logOut(b.String())
	}
}

func (l *standardLogger) logOut(message string) {
	l.outMu.Lock()
	fmt.Fprintf(l.stdout, "%s\n", message)
	l.outMu.Unlock()
}

func (l *standardLogger) logErr(message string) {
	l.errMu.Lock()
	fmt.Fprintf(l.stderr, "%s\n", message)
	l.errMu.Unlock()
}
