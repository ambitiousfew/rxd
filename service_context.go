package rxd

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ServiceContext all services will require a config as a *ServiceContext in their service struct.
// This config contains preconfigured shutdown channel,
type ServiceContext struct {
	Ctx       context.Context
	cancelCtx context.CancelFunc

	name string

	service    Service
	opts       *serviceOpts
	dependents map[State][]*ServiceContext

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

// ShutdownSignal returns the channel the side implementing the service should use and watch to be notified
// when the daemon/manager are attempting to shutdown services.
func (sc *ServiceContext) ShutdownSignal() <-chan struct{} {
	return sc.Ctx.Done()
}

// ChangeState returns the channel the service listens for state changes of the service it depends on
// defined by UsingServiceNotify option on creation of the ServiceContext.
func (sc *ServiceContext) ChangeState() chan State {
	return sc.stateC
}

// AddDependentService adds a service that depends on the current service and the states the dependent service is interested in.
func (sc *ServiceContext) AddDependentService(s *ServiceContext, states []State) error {
	if sc == s {
		return fmt.Errorf("cannot add service %s as a dependent service to itself", sc.name)
	}

	for _, state := range states {
		children, ok := sc.dependents[state]
		if !ok {
			sc.dependents[state] = []*ServiceContext{s}
			continue
		}

		children = append(children, s)
		sc.dependents[state] = children
	}
	return nil
}

// NotifyStateChange takes a state and iterates over all child services added via UsingServiceNotify, if any
// to notify them of the state change that occured against the service they subscribed to watch.
func (sc *ServiceContext) notifyStateChange(state State) {
	// If we dont have any services to notify, dont try.
	if len(sc.dependents) == 0 {
		return
	}

	svcs, ok := sc.dependents[state]
	if !ok {
		return
	}

	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()

	for _, svc := range svcs {
		if !svc.isShutdown {
			select {
			case <-timer.C:
				sc.LogDebug(fmt.Sprintf("could not inform %s of state change to %s, may not be watching", svc.name, state))
				continue
			case svc.stateC <- state:
				// attempt to send down channel, timeout if takes longer than 500ms
				sc.LogDebug(fmt.Sprintf("notification sent to dependent: %s", svc.name))
			}
			timer.Reset(250 * time.Millisecond)
		}
	}
}

func (sc *ServiceContext) setIsStopped(value bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.isStopped = value
}

func (sc *ServiceContext) setLogChannel(logC chan LogMessage) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.logC = logC
}

func (sc *ServiceContext) shutdown() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if !sc.isShutdown {
		close(sc.shutdownC)
		close(sc.stateC)
		sc.cancelCtx()
		sc.isShutdown = true
	}
}

// LogInfo takes a string message and sends it down the logC channel as a LogMessage type with log level of Info
func (sc *ServiceContext) LogInfo(message string) {
	sc.logC <- NewLog(serviceLog(sc, message), Info)
}

// LogInfof takes a string message and variadic params and sends it into Sprintf to be passed down the logC channel.
func (sc *ServiceContext) LogInfof(msg string, v ...any) {
	sc.logC <- NewLog(serviceLog(sc, fmt.Sprintf(msg, v...)), Info)
}

// LogDebug takes a string message and sends it down the logC channel as a LogMessage type with log level of Debug
func (sc *ServiceContext) LogDebug(message string) {
	sc.logC <- NewLog(serviceLog(sc, message), Debug)
}

// LogDebugf takes a string message and variadic params and sends it into Sprintf to be passed down the logC channel.
func (sc *ServiceContext) LogDebugf(msg string, v ...any) {
	sc.logC <- NewLog(serviceLog(sc, fmt.Sprintf(msg, v...)), Debug)
}

// LogError takes a string message and sends it down the logC channel as a LogMessage type with log level of Error
func (sc *ServiceContext) LogError(message string) {
	sc.logC <- NewLog(serviceLog(sc, message), Error)
}

// LogErrorf takes a string message and variadic params and sends it into Sprintf to be passed down the logC channel.
func (sc *ServiceContext) LogErrorf(msg string, v ...any) {
	sc.logC <- NewLog(serviceLog(sc, fmt.Sprintf(msg, v...)), Error)
}

// serviceLog is a helper that prefixes log string messages with the service name
func serviceLog(sc *ServiceContext, message string) string {
	return fmt.Sprintf("%s %s", sc.name, message)
}
