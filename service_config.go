package rxd

import (
	"sync"
)

// ServiceConfig all services will require a config as a *ServiceConfig in their service struct.
// This config contains preconfigured shutdown channel,
type ServiceConfig struct {
	opts *serviceOpts

	// ShutdownC is provided to each service to give the ability to watch for a shutdown signal.
	ShutdownC chan struct{}

	StateC chan State

	// Logging channel for manage to attach to services to use
	logC chan LogMessage

	// isStopped is a flag to tell is if we have been asked to run the Stop state
	isStopped bool
	// isShutdown is a flag that is true if close() has been called on the ShutdownC for the service in manager shutdown method
	isShutdown bool
	// mu is primarily used for mutations against isStopped and isShutdown between manager and wrapped service logic
	mu sync.Mutex
}

// NotifyStateChange takes a state and iterates over all services added via UsingServiceNotify, if any
func (cfg *ServiceConfig) NotifyStateChange(state State) {
	// If we dont have any services to notify, dont try.
	if cfg.opts.serviceNotify == nil {
		return
	}

	cfg.opts.serviceNotify.notify(state, cfg.logC)
}

func (cfg *ServiceConfig) setIsStopped(value bool) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	cfg.isStopped = value
}

func (cfg *ServiceConfig) setIsShutdown(value bool) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	cfg.isShutdown = value
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
	opts := &serviceOpts{
		// RestartPolicy:  Always,
		// RestartTimeout: 3 * time.Second,
		runPolicy: RunUntilStoppedPolicy,
	}

	// Apply all functional options to update defaults.
	for _, option := range options {
		option(opts)
	}

	return &ServiceConfig{
		ShutdownC:  make(chan struct{}),
		StateC:     make(chan State),
		opts:       opts,
		isStopped:  true,
		isShutdown: false,
	}
}
