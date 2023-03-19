package rxd

// ServiceConfig all services will require a config as a *ServiceConfig in their service struct.
// This config contains preconfigured shutdown channel,
type ServiceConfig struct {
	ShutdownC chan struct{}
	Opts      *ServiceOpts

	// Logging channel for manage to attach to services to use
	logC chan LogMessage
	// Whether the service is currently in a stopped state or not
	isStopped bool
}

// LogInfo takes a string message and sends it down the logC channel as a LogMessage type with log level of Info
func (cfg *ServiceConfig) LogInfo(message string) {
	cfg.logC <- NewLog(message, Info)
}

// LogDebug takes a string message and sends it down the logC channel as a LogMessage type with log level of Debug
func (cfg *ServiceConfig) LogDebug(message string) {
	cfg.logC <- NewLog(message, Debug)
}

// LogError takes a string message and sends it down the logC channel as a LogMessage type with log level of Error
func (cfg *ServiceConfig) LogError(message string) {
	cfg.logC <- NewLog(message, Error)
}

// NewServiceConfig will apply all options in the order given prior to creating the ServiceConfig instance created.
func NewServiceConfig(options ...ServiceOption) *ServiceConfig {
	// Default policy to restart immediately (3s) and always try to restart itself.
	opts := &ServiceOpts{
		// RestartPolicy:  Always,
		// RestartTimeout: 3 * time.Second,
		RunPolicy: RunUntilStoppedPolicy,
	}

	// Apply all functional options to update defaults.
	for _, option := range options {
		option(opts)
	}

	return &ServiceConfig{
		ShutdownC: make(chan struct{}),
		Opts:      opts,
		isStopped: true,
	}
}
