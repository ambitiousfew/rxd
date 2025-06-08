package journald

// Option defines a functional option for configuring the journaldHandler.
type Option func(h *journaldHandler)

// WithSeverityPrefix enables or disables the severity prefix in the log messages.
// The severity prefix is useful when using the journald driver within a docker container.
// If enabled, the log messages will start with a severity level in angle brackets (e.g., <6> for NOTICE).
func WithSeverityPrefix(enabled bool) Option {
	return func(h *journaldHandler) {
		h.severityPrefix = enabled
	}
}
