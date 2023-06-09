package rxd

// RunPolicy service option type representing the run policy of a given service
// basically controlling different ways of stopping a service like running only once when it succeeds
// without an error on Run
type RunPolicy string

const (
	// RunUntilStoppedPolicy will continue to run the service until a StopState is returned at some point
	RunUntilStoppedPolicy RunPolicy = "until_stopped"
	// RetryUntilSuccessPolicy will continue to re-run the service as long fails happen, use for running a service once successfully
	RetryUntilSuccessPolicy RunPolicy = "retry_until_success"
	// RunOncePolicy will only allow the a single Run to take place regardless of success/failure
	RunOncePolicy RunPolicy = "run_once"
)

// ServiceOption are simply using an Option pattern to customize options
// such as restart policies, timeouts for a given service and how it should run.
type ServiceOption func(*serviceOpts)

// NewServiceOpts will apply all options in the order given and return the final options back.
func NewServiceOpts(options ...ServiceOption) *serviceOpts {
	// Default runPolicy unless overridden
	opts := &serviceOpts{
		runPolicy: RunUntilStoppedPolicy,
		logger:    nil,
	}

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

// UsingLogger applies a given logger that meets the Logging interface to the ServiceOption instance
func UsingLogger(logger Logging) ServiceOption {
	return func(so *serviceOpts) {
		so.logger = logger
	}
}

// UsingDefaultLogger applies new logging instance as the logger with a customized log severity level and flags to use.
func UsingDefaultLogger(severity LogSeverity, flags int) ServiceOption {
	return func(so *serviceOpts) {
		so.logger = NewLogger(severity, flags)
	}
}

// ServiceOpts will allow for customizations of how a service runs and should always have
// a reasonable default to fallback if the case one isnt provided.
// This would be set by the ServiceConfig upon creation.
type serviceOpts struct {
	runPolicy RunPolicy
	logger    Logging
}
