package rxd

import (
	"bytes"
	"os"
	"sync"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

type DaemonLog struct {
	Name    string
	Level   log.Level
	Message string
	Fields  []log.Field
}

func (l DaemonLog) String() string {
	return l.Message
}

type daemonLogHandler struct {
	enabled  bool
	filepath string
	limit    uint64
	total    uint64
	file     *os.File
	mu       sync.RWMutex
}

func (h *daemonLogHandler) Handle(level log.Level, message string, fields []log.Field) {
	h.mu.RLock()
	if !h.enabled {
		h.mu.RUnlock()
		return
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.enabled && h.file == nil {
		f, err := os.OpenFile(h.filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			h.enabled = false
			return
		}
		h.file = f
	}

	// Log the message to the file
	var b bytes.Buffer
	b.WriteString(time.Now().Format("2006-01-02 15:04:05") + " ")
	b.WriteString(level.String() + ": ")
	b.WriteString(message)

	for _, f := range fields {
		b.WriteString(" ")
		b.WriteString(f.Key)
		b.WriteString("=")
		b.WriteString(f.Value)
	}
	b.WriteString("\n")

	msg := b.Bytes()

	if h.total+uint64(len(msg)) > h.limit {
		// if the length of the message is going to push us past the limit, truncate the file before next write.
		err := h.file.Truncate(0)
		if err != nil {
			return
		}
	}

	// Write the log message to the file
	n, err := h.file.Write(msg)
	h.total += uint64(n)
	if err != nil {
		return
	}
}

func (h *daemonLogHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.file != nil {
		err := h.file.Close()
		if err != nil {
			return err
		}
		h.file = nil
	}
	return nil
}

type noopLogger struct{}

func (n noopLogger) Log(level log.Level, message string, fields ...log.Field) {}
func (n noopLogger) SetLevel(level log.Level)                                 {}
