package log

import (
	"io"
	"os"
)

// HandlerOption is a functional option type for configuring a Handler.
type HandlerOption func(*defaultHandler)

// WithWriters allows customization of the stdout and stderr writers to use for the log message.
// if stdout or stderr is nil it will default to os.Stdout and os.Stderr respectively.
func WithWriters(stdout, stderr io.Writer) HandlerOption {
	return func(h *defaultHandler) {
		if stdout != nil {
			h.stdout = os.Stdout
		}
		if stderr != nil {
			h.stderr = os.Stderr
		}

		h.stdout = stdout
		h.stderr = stderr
	}
}

// WithStdOutWriter allows customization of the stdout writer to use log levels of NOTICE or higher.
// if stdout is nil it will default to os.Stdout
func WithStdOutWriter(stdout io.Writer) HandlerOption {
	return func(h *defaultHandler) {
		if stdout != nil {
			h.stdout = os.Stdout
		}

		h.stdout = stdout
	}
}

// WithStdErrWriter allows customization of the stderr writer to use log levels of WARNING or lower.
// if stderr is nil it will default to os.Stderr
func WithStdErrWriter(stderr io.Writer) HandlerOption {
	return func(h *defaultHandler) {
		if stderr != nil {
			h.stderr = os.Stderr
		}

		h.stderr = stderr
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
