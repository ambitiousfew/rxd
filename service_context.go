package rxd

import (
	"fmt"
	"sync"
)

// ServiceContext all services will require a config as a *ServiceContext in their service struct.
// This config contains preconfigured shutdown channel,
type ServiceContext struct {
	name string

	opts *serviceOpts

	// ShutdownC is provided to each service to give the ability to watch for a shutdown signal.
	shutdownC chan struct{}

	stateC chan State

	// Logging channel for manage to attach to services to use
	logC chan LogMessage

	// isStopped is a flag to tell is if we have been asked to run the Stop state
	isStopped bool
	// isShutdown is a flag that is true if close() has been called on the ShutdownC for the service in manager shutdown method
	isShutdown bool
	// mu is primarily used for mutations against isStopped and isShutdown between manager and wrapped service logic
	mu sync.Mutex
}

// NewServiceContext will apply all options in the order given prior to creating the ServiceContext instance created.
func NewServiceContext(name string, options ...ServiceOption) *ServiceContext {
	opts := &serviceOpts{
		runPolicy: RunUntilStoppedPolicy,
	}

	// Apply all functional options to update defaults.
	for _, option := range options {
		option(opts)
	}

	return &ServiceContext{
		name:       name,
		shutdownC:  make(chan struct{}),
		stateC:     make(chan State),
		opts:       opts,
		isStopped:  true,
		isShutdown: false,
	}
}

// ShutdownSignal returns the channel the side implementing the service should use and watch to be notified
// when the daemon/manager are attempting to shutdown services.
func (ctx *ServiceContext) ShutdownSignal() chan struct{} {
	return ctx.shutdownC
}

// ChangeState returns the channel the service listens for state changes of the service it depends on
// defined by UsingServiceNotify option on creation of the ServiceContext.
func (ctx *ServiceContext) ChangeState() chan State {
	return ctx.stateC
}

// NotifyStateChange takes a state and iterates over all child services added via UsingServiceNotify, if any
// to notify them of the state change that occured against the service they subscribed to watch.
func (ctx *ServiceContext) notifyStateChange(state State) {
	// If we dont have any services to notify, dont try.
	if ctx.opts.serviceNotify == nil {
		return
	}

	ctx.opts.serviceNotify.notify(state, ctx.logC)
}

func (ctx *ServiceContext) setIsStopped(value bool) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.isStopped = value
}

func (ctx *ServiceContext) setLogChannel(logC chan LogMessage) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.logC = logC
}

func (ctx *ServiceContext) shutdown() {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if !ctx.isShutdown {
		close(ctx.shutdownC)
		close(ctx.stateC)
		ctx.isShutdown = true
	}
}

// LogInfo takes a string message and sends it down the logC channel as a LogMessage type with log level of Info
func (ctx *ServiceContext) LogInfo(message string) {
	ctx.logC <- NewLog(serviceLog(ctx, message), Info)
}

// LogDebug takes a string message and sends it down the logC channel as a LogMessage type with log level of Debug
func (ctx *ServiceContext) LogDebug(message string) {
	ctx.logC <- NewLog(serviceLog(ctx, message), Debug)
}

// LogError takes a string message and sends it down the logC channel as a LogMessage type with log level of Error
func (ctx *ServiceContext) LogError(message string) {
	ctx.logC <- NewLog(serviceLog(ctx, message), Error)
}

// serviceLog is a helper that prefixes log string messages with the service name
func serviceLog(ctx *ServiceContext, message string) string {
	return fmt.Sprintf("%s %s", ctx.name, message)
}
