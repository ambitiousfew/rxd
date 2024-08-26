package log

import (
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type defaultHandler struct {
	out io.Writer
	mu  sync.RWMutex

	disabled bool
	msgfmt   string
	timefmt  string
}

func NewHandler(opts ...HandlerOption) LogHandler {
	h := &defaultHandler{
		out:      os.Stdout,
		mu:       sync.RWMutex{},
		msgfmt:   "{time} [{level}] {message}",
		timefmt:  time.RFC3339,
		disabled: false,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
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

	h.mu.Lock()
	h.out.Write([]byte(out + "\n"))
	h.mu.Unlock()
}
