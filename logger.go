package rxd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Logger interface {
	With(group string) Logger
	// level 3
	Error(msg string, fields map[string]any)
	// level 4
	Warn(msg string, fields map[string]any)
	// level 5
	Notice(msg string, fields map[string]any)
	// level 6
	Info(msg string, fields map[string]any)
	// level 7
	Debug(msg string, fields map[string]any)
}

const (
	// ErrorLevel is the level for error messages
	LogLevelError LogLevel = iota + 3
	// WarnLevel is the level for warning messages
	LogLevelWarning
	// NoticeLevel is the level for notice messages
	LogLevelNotice
	// InfoLevel is the level for informational messages
	LogLevelInfo
	// DebugLevel is the level for debug messages
	LogLevelDebug
)

type LogLevel uint8

func (l LogLevel) String() string {
	switch l {
	case LogLevelError:
		return "ERROR"
	case LogLevelWarning:
		return "WARNING"
	case LogLevelNotice:
		return "NOTICE"
	case LogLevelInfo:
		return "INFO"
	case LogLevelDebug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

type logger struct {
	name    string
	group   string
	timefmt string
	level   LogLevel

	stdout io.Writer
	stderr io.Writer
	outMu  sync.Mutex
	errMu  sync.Mutex
}

type LoggerOption func(l *logger)

func UsingLogName(name string) LoggerOption {
	return func(l *logger) {
		l.name = name
	}
}

func UsingLogTimeFormat(format string) LoggerOption {
	return func(l *logger) {
		l.timefmt = format
	}
}

func UsingLogLevel(level LogLevel) LoggerOption {
	return func(l *logger) {
		l.level = level
	}
}

func NewLogger(stdout, stderr io.Writer, opts ...LoggerOption) Logger {
	l := &logger{
		name:    "",
		group:   "",
		timefmt: "",
		level:   LogLevelInfo,
		stdout:  stdout,
		stderr:  stderr,
		errMu:   sync.Mutex{},
		outMu:   sync.Mutex{},
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

func NewDefaultLogger(level LogLevel) Logger {
	l := &logger{
		name:    "",
		group:   "",
		timefmt: time.RFC3339,
		level:   level,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		errMu:   sync.Mutex{},
		outMu:   sync.Mutex{},
	}

	return l
}

func (l *logger) log(level LogLevel, msg string, fields map[string]any) {
	// if the logger level is less than level passed, we don't log
	if l.level < level {
		return
	}

	var b strings.Builder
	// if a time format is set for the logger, add the time to the message
	if l.timefmt != "" {
		b.WriteString(time.Now().Format(l.timefmt) + " ")
	}

	b.WriteString("[" + level.String() + "] ")
	// if a log name is set, add it to the message before the level
	if l.group != "" {
		b.WriteString(l.group + ": ")
	}

	b.WriteString(msg)

	for k, v := range fields {
		b.WriteString(" " + k + "=" + fmt.Sprintf("%v", v))
	}

	if l.name != "" {
		b.WriteString(" daemon=" + l.name)
	}

	message := b.String()
	switch level {
	case LogLevelError:
		l.logErr(message)
	default:
		l.logOut(message)
	}
}

func (l *logger) logOut(message string) {
	l.outMu.Lock()
	fmt.Fprintf(l.stdout, "%s\n", message)
	l.outMu.Unlock()
}

func (l *logger) logErr(message string) {
	l.errMu.Lock()
	fmt.Fprintf(l.stderr, "%s\n", message)
	l.errMu.Unlock()
}

func (l *logger) setLevel(level LogLevel) {
	// lock both writers before changing the level
	// neither can write until the level is set
	l.outMu.Lock()
	l.errMu.Lock()
	l.level = level
	l.outMu.Unlock()
	l.errMu.Unlock()
}

// With creates a new child logger with the given group and value
func (l *logger) With(group string) Logger {
	return &logger{
		name:    l.name,
		group:   group,
		timefmt: l.timefmt,
		level:   l.level,
		stdout:  l.stdout,
		stderr:  l.stderr,
		outMu:   sync.Mutex{},
		errMu:   sync.Mutex{},
	}
}

func (l *logger) Error(message string, fields map[string]interface{}) {
	l.log(LogLevelError, message, fields)
}

func (l *logger) Warn(message string, fields map[string]interface{}) {
	l.log(LogLevelWarning, message, fields)
}

func (l *logger) Notice(message string, fields map[string]interface{}) {
	l.log(LogLevelNotice, message, fields)
}

func (l *logger) Info(message string, fields map[string]interface{}) {
	l.log(LogLevelInfo, message, fields)
}

func (l *logger) Debug(message string, fields map[string]interface{}) {
	l.log(LogLevelDebug, message, fields)
}
