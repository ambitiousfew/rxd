package log

import (
	"io"
)

type HandlerOption func(*defaultHandler)

// WithWriter allows customization of the writer to use for the log message.
func WithWriter(out io.Writer) HandlerOption {
	return func(h *defaultHandler) {
		h.out = out
	}
}

// WithMessageFormat allows customization of the message format for the log message.
func WithMessageFormat(format string) HandlerOption {
	return func(l *defaultHandler) {
		l.msgfmt = format
	}
}

// WithTimeFormat allows customization of the time format for the log message.
func WithTimeFormat(format string) HandlerOption {
	return func(l *defaultHandler) {
		l.timefmt = format
	}
}

// WithEnabled sets the handler to be enabled or disabled
// if the handler is disabled, it will not log anything.
func WithEnabled(enabled bool) HandlerOption {
	return func(l *defaultHandler) {
		l.disabled = !enabled // flip the value
	}
}
