package rxd

import (
	"context"

	"golang.org/x/exp/slog"
)

const (
	// RunUntilStoppedPolicy will continue to run the service until a StopState is returned at some point
	RunUntilStoppedPolicy RunPolicy = iota
	// RetryUntilSuccessPolicy will continue to re-run the service as long fails happen, use for running a service once successfully
	RetryUntilSuccessPolicy
	// RunOncePolicy will only allow the a single Run to take place regardless of success/failure
	RunOncePolicy
)

// RunPolicy service option type representing the run policy of a given service
// basically controlling different ways of stopping a service like running only once when it succeeds
// without an error on Run
type RunPolicy int

func (r RunPolicy) String() string {
	switch r {
	case RunUntilStoppedPolicy:
		return "run_until_stopped"
	case RetryUntilSuccessPolicy:
		return "retry_until_success"
	case RunOncePolicy:
		return "run_once"
	default:
		return "unknown"
	}
}

// ServiceOption are simply using an Option pattern to customize options
// such as restart policies, timeouts for a given service and how it should run.
type ServiceOption func(*serviceOpts)

// NewServiceOpts will apply all options in the order given and return the final options back.
func NewServiceOpts(options ...ServiceOption) *serviceOpts {
	// Default runPolicy unless overridden
	opts := &serviceOpts{}

	// Apply all functional options to update defaults.
	for _, option := range options {
		option(opts)
	}

	// set reasonable defaults if not provided.
	if opts.ctx == nil {
		opts.ctx, opts.cancel = context.WithCancel(context.Background())
	}

	if opts.logHandler == nil {
		opts.logHandler = slog.Default().Handler()
	}

	return opts
}

// UsingRunPolicy applies a given policy to the ServiceOption instance
func UsingRunPolicy(policy RunPolicy) ServiceOption {
	return func(so *serviceOpts) {
		so.runPolicy = policy
	}
}

// UsingLogHandler applies a given slog Handler to the underlying service options
func UsingLogHandler(h slog.Handler) ServiceOption {
	return func(so *serviceOpts) {
		so.logHandler = h
	}
}

// UsingContext creates a new context using the given context as its parent context
func UsingContext(parent context.Context) ServiceOption {
	ctx, cancel := context.WithCancel(parent)
	return func(so *serviceOpts) {
		so.ctx = ctx
		so.cancel = cancel
	}
}

// ServiceOpts will allow for customizations of how a service runs and should always have
// a reasonable default to fallback if the case one isnt provided.
// This would be set by the ServiceConfig upon creation.
type serviceOpts struct {
	ctx    context.Context
	cancel context.CancelFunc

	runPolicy RunPolicy

	logHandler slog.Handler
}
