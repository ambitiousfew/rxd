package rxd

// serviceOpts will allow for customizations of how a service runs and should always have
// a reasonable default to fallback if the case one isnt provided.
// This would be set by the ServiceConfig upon creation.
type serviceOpts struct {
	runPolicy RunPolicy
}

type ServiceOption func(*serviceOpts)

// NewServiceOpts will apply all options in the order given and return the final options back.
func NewServiceOpts(options ...ServiceOption) *serviceOpts {
	// Default runPolicy unless overridden
	opts := &serviceOpts{}

	// Apply all functional options to update defaults.
	for _, option := range options {
		option(opts)
	}

	return opts
}

// UsingServiceRunPolicy applies a given policy to the ServiceOption instance
func UsingServiceRunPolicy(policy RunPolicy) ServiceOption {
	return func(so *serviceOpts) {
		so.runPolicy = policy
	}
}
