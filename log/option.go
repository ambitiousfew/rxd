package log

import (
	"io"
)

type HandlerOption func(*defaultHandler)

func WithWriter(out io.Writer) HandlerOption {
	return func(h *defaultHandler) {
		h.out = out
	}
}

func WithMessageFormat(format string) HandlerOption {
	return func(l *defaultHandler) {
		l.msgfmt = format
	}
}

func WithTimeFormat(format string) HandlerOption {
	return func(l *defaultHandler) {
		l.timefmt = format
	}
}
