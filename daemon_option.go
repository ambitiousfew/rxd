package rxd

import (
	"log/slog"
)

type DaemonOption func(*daemonOpts)

// NewDaemonOpts will apply all options in the order given and return the final options back.
func NewDaemonOpts(options ...DaemonOption) *daemonOpts {
	// Default runPolicy unless overridden
	opts := &daemonOpts{}

	// Apply all functional options to update defaults.
	for _, option := range options {
		option(opts)
	}

	return opts
}

// UsingLogger applies a given slog Logger to the underlying service options
func UsingLogger(l *slog.Logger) DaemonOption {
	return func(so *daemonOpts) {
		so.log = l
	}
}

// daemonOpts will allow for customizations of how a daemon runs and should always have
// a reasonable default to fallback if the case one isnt provided.
type daemonOpts struct {
	log *slog.Logger
}
