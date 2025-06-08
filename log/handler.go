package log

import (
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// NewHandler creates a new default Handler with the provided options.
// The default handler writes logs to os.Stdout and os.Stderr, with a default
// message format of "{time} [{level}] {message}" and a time format of time.RFC3339.
// If no options are provided, it will use these defaults.
func NewHandler(opts ...HandlerOption) Handler {
	h := &defaultHandler{
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		outMu:    sync.RWMutex{},
		errMu:    sync.RWMutex{},
		msgfmt:   "{time} [{level}] {message}",
		timefmt:  time.RFC3339,
		disabled: false,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

type defaultHandler struct {
	stdout   io.Writer
	stderr   io.Writer
	outMu    sync.RWMutex
	errMu    sync.RWMutex
	disabled bool
	msgfmt   string
	timefmt  string
}

func (h *defaultHandler) Handle(level Level, message string, fields []Field) {
	if h.disabled {
		// if the handler is disabled, we don't log anything
		return
	}
	// replace the main fields first.
	fmtMsg := strings.Replace(h.msgfmt, "{time}", time.Now().Format(h.timefmt), 1)
	fmtMsg = strings.Replace(fmtMsg, "{level}", level.String(), 1)
	fmtMsg = strings.Replace(fmtMsg, "{message}", message, 1)

	var b strings.Builder

	b.WriteString(fmtMsg)

	for _, field := range fields {
		b.WriteString(" " + field.Key + "=" + field.Value)
	}

	out := b.String()

	if level < LevelNotice {
		// anything warning(4) and lower goes to stderr
		h.writeErr(out)
	} else {
		// everything goes out to stdout
		h.writeOut(out)
	}
}

func (h *defaultHandler) writeOut(out string) {
	h.outMu.Lock()
	defer h.outMu.Unlock()
	h.stdout.Write([]byte(out + "\n"))
}

func (h *defaultHandler) writeErr(out string) {
	h.errMu.Lock()
	defer h.errMu.Unlock()
	h.stderr.Write([]byte(out + "\n"))
}
