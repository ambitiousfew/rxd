package journald

type Option func(h *journaldHandler)

func WithSeverityPrefix(enabled bool) Option {
	return func(h *journaldHandler) {
		h.severityPrefix = enabled
	}
}
