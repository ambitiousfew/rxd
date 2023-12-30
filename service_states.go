package rxd

import (
	"context"
	"fmt"

	"github.com/ambitiousfew/intracom"
)

// StateUpdate reflects any given update of lifecycle state at a given time.
type StateUpdate struct {
	Name  string
	State State
}

// States is a map of service name to service state which
// reflects the service name and its lifecycle state.
type States map[string]State

func AnyServicesEnterState(sc *ServiceContext, target State, serviceNames ...string) (<-chan States, context.CancelFunc) {
	if sc.iStates == nil {
		sc.Log.Warn("AllServicesEnteredState called with nil iStates")
		return nil, func() {} // noop cancel
	}
	checkC := make(chan States, 1)

	ctx, cancel := context.WithCancel(sc.Ctx)

	go func() {
		// subscribe to the internal states on behalf of the service context given.
		consumer := internalEnterStatesConsumer(sc.Name, target)
		subscriberC, unsubscribe := sc.iStates.Subscribe(&intracom.SubscriberConfig{
			Topic:         internalServiceStates,
			ConsumerGroup: consumer,
			BufferSize:    1,
			BufferPolicy:  intracom.DropOldest,
		})

		defer close(checkC)
		defer unsubscribe()

		for {
			select {
			case <-ctx.Done():
				return

			case states, open := <-subscriberC:
				if !open {
					return
				}

				interestedServices := make(States)
				for _, name := range serviceNames {
					if val := states[name]; val == target {
						// build states map of only services we care about.
						interestedServices[name] = val
					}
				}

				// if we found all those we care about.
				if len(interestedServices) > 0 {
					checkC <- interestedServices // send out the states
				}
			}

		}

	}()

	return checkC, cancel
}

func AllServicesEnterState(sc *ServiceContext, target State, serviceNames ...string) (<-chan States, context.CancelFunc) {
	if sc.iStates == nil {
		sc.Log.Warn("AllServicesEnteredState called with nil iStates")
		return nil, func() {} // noop cancel
	}
	checkC := make(chan States, 1)

	ctx, cancel := context.WithCancel(sc.Ctx)

	go func() {
		// subscribe to the internal states on behalf of the service context given.
		consumer := internalEnterStatesConsumer(sc.Name, target)
		subscriberC, unsubscribe := sc.iStates.Subscribe(&intracom.SubscriberConfig{
			Topic:         internalServiceStates,
			ConsumerGroup: consumer,
			BufferSize:    1,
			BufferPolicy:  intracom.DropOldest,
		})

		defer close(checkC)
		defer unsubscribe()

		for {
			select {
			case <-ctx.Done():
				return

			case states, open := <-subscriberC:
				if !open {
					return
				}

				interestedServices := make(States)
				for _, name := range serviceNames {
					if val := states[name]; val == target {
						// build states map of only services we care about.
						interestedServices[name] = val
					}
				}

				// if we found all those we care about.
				if len(interestedServices) == len(serviceNames) {
					checkC <- interestedServices // send out the states
				}
			}

		}

	}()

	return checkC, cancel
}

// AnyServicesExitState
func AnyServicesExitState(sc *ServiceContext, target State, serviceNames ...string) (<-chan States, context.CancelFunc) {
	if sc.iStates == nil {
		sc.Log.Warn("AllServicesEnteredState called with nil iStates")
		return nil, func() {} // noop cancel
	}
	checkC := make(chan States, 1)

	ctx, cancel := context.WithCancel(sc.Ctx)

	go func() {
		// subscribe to the internal states on behalf of the service context given to _rxd.<state>.states.<service_name>
		consumer := internalExitStatesConsumer(sc.Name, target)
		subscriberC, unsubscribe := sc.iStates.Subscribe(&intracom.SubscriberConfig{
			Topic:         internalServiceStates,
			ConsumerGroup: consumer,
			BufferSize:    1,
			BufferPolicy:  intracom.DropOldest,
		})

		defer close(checkC)
		defer unsubscribe()

		for {
			select {
			case <-ctx.Done():
				return

			case states, open := <-subscriberC:
				if !open {
					return
				}

				interestedExits := make(States)
				for _, name := range serviceNames {
					if state := states[name]; state != target {
						// build the map of exited states to send out.
						interestedExits[name] = state
					}
				}

				if len(interestedExits) > 0 {
					// if we found ANY that we care about.
					checkC <- interestedExits
				}
			}

		}

	}()

	return checkC, cancel
}

func AllServicesStates(sc *ServiceContext) (<-chan States, context.CancelFunc) {
	if sc.iStates == nil {
		sc.Log.Warn("AllServicesEnteredState called with nil iStates")
		return nil, func() {} // noop cancel
	}
	checkC := make(chan States, 1)

	ctx, cancel := context.WithCancel(sc.Ctx)

	go func() {
		// subscribe to the internal states on behalf of the service context given to _rxd.<state>.states.<service_name>
		consumer := internalAllStatesConsumer(sc.Name)
		subscriberC, unsubscribe := sc.iStates.Subscribe(&intracom.SubscriberConfig{
			Topic:         internalServiceStates,
			ConsumerGroup: consumer,
			BufferSize:    1,
			BufferPolicy:  intracom.DropOldest,
		})
		defer func() {
			close(checkC)
			unsubscribe()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case states, open := <-subscriberC:
				if !open {
					return
				}

				select {
				case <-ctx.Done():
					return
				case checkC <- states:
				}
			}
		}

	}()

	return checkC, cancel
}

func internalAllStatesConsumer(name string) string {
	return fmt.Sprintf("%s.%s", internalServiceAllStates, name)
}

func internalEnterStatesConsumer(name string, state State) string {
	return fmt.Sprintf("%s.%s.%s", internalServiceEnterStates, state, name)
}

func internalExitStatesConsumer(name string, state State) string {
	return fmt.Sprintf("%s.%s.%s", internalServiceExitStates, state, name)
}
