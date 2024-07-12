package journald

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ambitiousfew/rxd/log"
)

type journaldLogger struct {
	severityPrefix bool
	fields         []log.Field
	level          log.Level
	stdout         io.Writer
	stderr         io.Writer
	lvlMu          sync.RWMutex // mutex for level and fields
	outMu          sync.RWMutex // mutex for stdout writer
	errMu          sync.RWMutex // mutex for stderr writer
}

// NewLogger creates a new instance of the journal logger that logs only what is necessary
// to journal and allows for the journal to handle opening the stdout and stderr streams and tagging
// timestamps and program name.
func NewLogger(level log.Level, opts ...Option) log.Logger {
	jlogger := &journaldLogger{
		severityPrefix: false,
		level:          level,
		fields:         []log.Field{},
		stdout:         os.Stdout,
		stderr:         os.Stderr,
		lvlMu:          sync.RWMutex{},
		outMu:          sync.RWMutex{},
		errMu:          sync.RWMutex{},
	}

	for _, opt := range opts {
		opt(jlogger)
	}

	return jlogger
}

func (l *journaldLogger) With(fields ...log.Field) log.Logger {
	extraFields := append(l.fields, fields...)

	return &journaldLogger{
		severityPrefix: l.severityPrefix,
		level:          l.level,
		fields:         extraFields,
		stdout:         l.stdout,
		stderr:         l.stderr,
		lvlMu:          sync.RWMutex{},
		outMu:          sync.RWMutex{},
		errMu:          sync.RWMutex{},
	}
}

func (l *journaldLogger) SetLevel(level log.Level) {
	l.lvlMu.Lock()
	l.level = level
	l.lvlMu.Unlock()
}

func (l *journaldLogger) Log(level log.Level, msg string, fields ...log.Field) {
	l.lvlMu.RLock()
	// if the logger level is less than level passed, we don't log
	if l.level < level {
		l.lvlMu.RUnlock()
		return
	}
	l.lvlMu.RUnlock()

	var b strings.Builder
	// if a log name is set, add it to the message before the level
	if l.severityPrefix {
		// NOTE: this is to support the severity prefix when using the journald driver within a docker container.
		b.WriteString("<" + strconv.Itoa(int(level)) + ">")
	}
	b.WriteString("[" + level.String() + "] ")
	b.WriteString(msg)

	allFields := append(l.fields, fields...)
	// write all the logger fields to the message first
	for _, field := range allFields {
		b.WriteString(" " + field.Key + "=" + field.Value)
	}

	message := b.String()
	switch level {
	case log.LevelError:
		l.logErr(message)
	default:
		l.logOut(message)
	}
}

func (l *journaldLogger) logOut(message string) {
	l.outMu.Lock()
	fmt.Fprintf(l.stdout, "%s\n", message)
	l.outMu.Unlock()
}

func (l *journaldLogger) logErr(message string) {
	l.errMu.Lock()
	fmt.Fprintf(l.stderr, "%s\n", message)
	l.errMu.Unlock()
}
