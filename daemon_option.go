package rxd

import (
	"log/slog"
)

type DaemonOption func(*daemonOpts)

// NewServiceOpts will apply all options in the order given and return the final options back.
func NewDaemonOpts(options ...ServiceOption) *serviceOpts {
	// Default runPolicy unless overridden
	opts := &serviceOpts{}

	// Apply all functional options to update defaults.
	for _, option := range options {
		option(opts)
	}

	return opts
}

// UsingRunPolicy applies a given policy to the ServiceOption instance
func UsingRunPolicy(policy RunPolicy) ServiceOption {
	return func(so *serviceOpts) {
		so.runPolicy = policy
	}
}

// UsingLogger applies a given slog Logger to the underlying service options
func UsingLogger(l *slog.Logger) ServiceOption {
	return func(so *serviceOpts) {
		so.log = l
	}
}

// daemonOpts will allow for customizations of how a daemon runs and should always have
// a reasonable default to fallback if the case one isnt provided.
type daemonOpts struct {
	log *slog.Logger
}
