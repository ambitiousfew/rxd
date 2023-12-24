package rxd

import (
	"encoding/json"
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

func AllServicesEnteredState(sc *ServiceContext, target State, serviceNames ...string) (<-chan States, chan<- struct{}) {
	checkC := make(chan States, 1)

	doneC := make(chan struct{})

	go func() {
		// subscribe to the internal states on behalf of the service context given.
		consumer := internalEnterStatesConsumer(sc.Name, target)
		subscriptionC, unsubscribe := sc.intracom.Subscribe(&intracom.SubscriberConfig{
			Topic:         internalServiceStates,
			ConsumerGroup: consumer,
			BufferSize:    1,
			BufferPolicy:  intracom.DropNone,
		})

		defer func() {
			close(checkC)
			unsubscribe()
		}()

		for {
			select {
			case <-doneC:
				// signaled stop
				return
			case stateBytes, open := <-subscriptionC:
				if !open {
					// channel closed
					return
				}

				var states States
				err := json.Unmarshal(stateBytes, &states)
				if err != nil {
					sc.Log.Warn("rxd failed at unmarshalling the states", "func", "AllServicesEnteredState", "service", sc.Name)
					return
				}

				interestedServices := make(States)
				for _, name := range serviceNames {
					if state := states[name]; state == target {
						// build the map of entered states to send out.
						interestedServices[name] = state
					}
				}

				if len(interestedServices) == len(serviceNames) {

					select {
					case <-doneC:
						return
					case checkC <- interestedServices:
						// if we found all those we care about.
					}
				}
			}

		}

	}()

	return checkC, doneC
}

// AnyServicesExitState
func AnyServicesExitState(sc *ServiceContext, target State, serviceNames ...string) (<-chan States, chan<- struct{}) {
	checkC := make(chan States, 1)

	doneC := make(chan struct{})
	// ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// subscribe to the internal states on behalf of the service context given to _rxd.<state>.states.<service_name>
		consumer := internalExitStatesConsumer(sc.Name, target)
		subscriptionC, unsubscribe := sc.intracom.Subscribe(&intracom.SubscriberConfig{
			Topic:         internalServiceStates,
			ConsumerGroup: consumer,
			BufferSize:    1,
			BufferPolicy:  intracom.DropNone,
		})

		defer func() {
			close(checkC)
			unsubscribe()
		}()

		for {
			select {
			case <-doneC:
				return
			case stateBytes, open := <-subscriptionC:
				if !open {
					// channel closed
					return
				}

				var states States
				err := json.Unmarshal(stateBytes, &states)
				if err != nil {
					sc.Log.Warn("rxd failed at unmarshalling the states", "func", "AnyServiceExitsState", "service", sc.Name)
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
					select {
					case <-doneC:
						return
					case checkC <- interestedExits:
						// if we found ANY that we care about.
					}

				}
			}

		}

	}()

	return checkC, doneC
}

// AllServicesStates
func AllServicesStates(sc *ServiceContext) (<-chan States, chan<- struct{}) {
	checkC := make(chan States, 1)
	doneC := make(chan struct{})

	go func() {
		// subscribe to the internal states on behalf of the service context given to _rxd.<state>.states.<service_name>
		consumer := internalAllStatesConsumer(sc.Name)
		subscriptionC, unsubscribe := sc.intracom.Subscribe(&intracom.SubscriberConfig{
			Topic:         internalServiceStates,
			ConsumerGroup: consumer,
			BufferSize:    1,
			BufferPolicy:  intracom.DropNone,
		})
		defer func() {
			close(checkC)
			unsubscribe()
		}()

		for {
			select {
			case <-doneC:
				return
			case stateBytes, open := <-subscriptionC:
				if !open {
					// channel closed
					return
				}
				var states States
				err := json.Unmarshal(stateBytes, &states)
				if err != nil {
					sc.Log.Warn("rxd failed at unmarshalling the states", "func", "AllServicesStates", "service", sc.Name)
					return
				}

				// try to send and while we wait still watch for cancel
				select {
				case <-doneC:
					return
				case checkC <- states:
				}
			}
		}

	}()

	return checkC, doneC
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
