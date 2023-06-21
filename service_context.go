package rxd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ambitiousfew/intracom"
)

// ServiceContext all services will require a config as a *ServiceContext in their service struct.
// This config contains preconfigured shutdown channel,
type ServiceContext struct {
	Name string
	Ctx  context.Context
	Log  Logging

	cancelCtx context.CancelFunc

	service Service
	opts    *serviceOpts

	dependents map[*ServiceContext]map[State]struct{}
	intracom   *intracom.Intracom[[]byte]

	isDependent bool

	// stateC all services report their own state changes up this channel
	stateC chan State
	// stateChangeC all dependent services receive parent state changes on this channel.
	stateChangeC chan State

	// isStopped is a flag to tell is if we have been asked to run the Stop state
	isStopped bool

	// ensure a service shutdown cannot be called twice
	stopCalled     atomic.Int32
	shutdownCalled atomic.Int32

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
	return sc.stateChangeC
}

// IntracomSubscribe subscribes this service to inter-service communication by its topic name.
// A channel is returned to receive published messages from another service.
// Standardized on slice of bytes since it allows us to easily json unmarshal into struct if needed.
func (sc *ServiceContext) IntracomSubscribe(topic string, id string) <-chan []byte {
	return sc.intracom.Subscribe(topic, id, 1)
}

// IntracomUnsubscribe unsubscribes this service from inter-service communication by its topic name
func (sc *ServiceContext) IntracomUnsubscribe(topic string, id string) {
	sc.intracom.Unsubscribe(topic, id)
}

// IntracomPublish will publish the bytes message to the topic, only if there is subscribed interest.
func (sc *ServiceContext) IntracomPublish(topic string, message []byte) {
	if strings.HasPrefix(topic, "_") {
		sc.Log.Warnf("%s is trying to publish using a reserved topic prefix '%s', auto-removing prefix")
		topic = strings.Replace(topic, "_", "", 1)
	}
	sc.intracom.Publish(topic, message)
}

// AddDependentService adds a service that depends on the current service and the states the dependent service is interested in.
func (sc *ServiceContext) AddDependentService(s *ServiceContext, states ...State) error {
	if sc == s {
		// a parent service should not be trying to add itself as a dependent to itself.
		return fmt.Errorf("cannot add service %s as a dependent service to itself", sc.Name)
	}

	if len(states) == 0 {
		// since states is variadic, make sure we have at least 1 otherwise why bother calling this method.
		return fmt.Errorf("cannot add dependent service %s with no interested states", sc.Name)
	}

	if len(sc.dependents) == 0 {
		// ensure our map isnt a nil map.
		sc.dependents = make(map[*ServiceContext]map[State]struct{})
	}

	// hold onto the states that we were interested in.
	interested := make(map[State]struct{})

	for _, state := range states {
		interested[state] = struct{}{}
	}

	s.isDependent = true
	// creating a mapping of dependent services to the states they claimed to be interested in.
	sc.dependents[s] = interested

	return nil
}

// NotifyStateChange takes a state and iterates over all child services added via UsingServiceNotify, if any
// to notify them of the state change that occured against the service they subscribed to watch.
func (sc *ServiceContext) notifyStateChange(state State) {
	// If we dont have any services to notify, dont try.
	if len(sc.dependents) == 0 {
		return
	}

	sc.Log.Debugf("%s notifying dependents of next state, %s", sc.Name, string(state))

	select {
	case <-sc.Ctx.Done():
		// parent service is shutting down, exit.
		return
	case sc.stateC <- state:
		// send parent state up channel so notifier routine can hold it
		// to send down to all children that care and wait.
		return
	}

}

func (sc *ServiceContext) hasStopped() bool {
	return sc.stopCalled.Load() == 1
}

func (sc *ServiceContext) hasShutdown() bool {
	// 0 has not shutdown, 1 has shutdown
	return sc.shutdownCalled.Load() == 1
}

func (sc *ServiceContext) setIntracom(intracom *intracom.Intracom[[]byte]) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.intracom = intracom
}

func (sc *ServiceContext) shutdown() {
	if sc.shutdownCalled.Swap(1) == 1 {
		// if we swap and the old value is already 1, shutdown was already called.
		return
	}

	// if the current service has dependents, shut them down first.
	for dsvc := range sc.dependents {
		sc.Log.Debugf("signaling shutdown of dependent service: %s", dsvc.Name)
		dsvc.shutdown()
	}

	sc.Log.Debugf("%s shutting down...", sc.Name)
	sc.cancelCtx()
	sc.shutdownCalled.Swap(1)
}
