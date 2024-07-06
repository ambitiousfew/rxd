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
	level          log.Level
	severityPrefix bool

	stdout io.Writer
	outMu  sync.Mutex

	stderr io.Writer
	errMu  sync.Mutex
}

// NewJournaldLogger creates a new instance of the journal logger that logs only what is necessary
// to journal and allows for the journal to handle opening the stdout and stderr streams and tagging
// timestamps and program name.
func New(level log.Level, opts ...Option) log.Logger {
	jlogger := &journaldLogger{
		severityPrefix: false,
		level:          level,
		stdout:         os.Stdout,
		stderr:         os.Stderr,
		outMu:          sync.Mutex{},
		errMu:          sync.Mutex{},
	}

	for _, opt := range opts {
		opt(jlogger)
	}

	return jlogger
}

func (l *journaldLogger) SetLevel(level log.Level) {
	l.level = level
}

func (l *journaldLogger) Log(level log.Level, msg string, args ...any) {
	// if the logger level is less than level passed, we don't log
	if l.level < level {
		return
	}

	// TODO: args must be key-value pairs so they come in pairs of two.
	// dig into slow and see if there is a better way to handle this.
	// maybe it would be better as a type that can be passed in.
	if len(args) > 0 && len(args)%2 != 0 {
		l.logErr("invalid number of arguments: " + strconv.Itoa(len(args)) + ", must be key-value pairs")
		return
	}

	var b strings.Builder
	// if a log name is set, add it to the message before the level
	if l.severityPrefix {
		// NOTE: this is to support the severity prefix when using the journald driver within a docker container.
		b.WriteString("<" + strconv.Itoa(int(level)) + ">")
	}
	b.WriteString("[" + level.String() + "] ")
	b.WriteString(msg)

	for i, arg := range args {
		val := arg.(string)
		if i%2 == 0 {
			b.WriteString(" " + val + "=")
		} else {
			b.WriteString(val)
		}
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
