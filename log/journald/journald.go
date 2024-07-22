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

type journaldHandler struct {
	severityPrefix bool
	lvlMu          sync.RWMutex // mutex for level and fields
	stdout         io.Writer
	outMu          sync.RWMutex // mutex for stdout writer
	stderr         io.Writer
	errMu          sync.RWMutex // mutex for stderr writer
}

func NewHandler(opts ...Option) log.LogHandler {
	h := &journaldHandler{
		severityPrefix: false,
		lvlMu:          sync.RWMutex{},
		stdout:         os.Stdout,
		outMu:          sync.RWMutex{},
		stderr:         os.Stderr,
		errMu:          sync.RWMutex{},
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

func (h *journaldHandler) Handle(level log.Level, message string, fields []log.Field) {
	var b strings.Builder
	// if a log name is set, add it to the message before the level
	if h.severityPrefix {
		// NOTE: this is to support the severity prefix when using the journald driver within a docker container.
		b.WriteString("<" + strconv.Itoa(int(level)) + ">")
	}
	b.WriteString("[" + level.String() + "] ")
	b.WriteString(message)

	// allFields := append(h.fields, fields...)
	// write all the logger fields to the message first
	for _, field := range fields {
		b.WriteString(" " + field.Key + "=" + field.Value)
	}

	out := b.String()
	switch level < log.LevelWarning {
	case true:
		// logs to stderr for error and lower (higher severity)
		h.logErr(out)
	default:
		// logs to stdout for warning and above (lower severity)
		h.logOut(out)
	}
}

func (h *journaldHandler) logOut(message string) {
	h.outMu.Lock()
	fmt.Fprintf(h.stdout, "%s\n", message)
	h.outMu.Unlock()
}

func (h *journaldHandler) logErr(message string) {
	h.errMu.Lock()
	fmt.Fprintf(h.stderr, "%s\n", message)
	h.errMu.Unlock()
}
