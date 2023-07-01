package rxd

import (
	"context"
	"encoding/json"
	"fmt"
)

// StateUpdate reflects any given update of lifecycle state at a given time.
type StateUpdate struct {
	Name  string
	State State
}

// States is a map of service name to service state which
// reflects the service name and its lifecycle state.
type States map[string]State

func AllServicesEnteredState(sc *ServiceContext, target State, serviceNames ...string) (<-chan States, context.CancelFunc) {
	checkC := make(chan States, 1)

	ctx, cancel := context.WithCancel(sc.Ctx)

	go func() {
		// subscribe to the internal states on behalf of the service context given.
		consumer := internalEnterStatesConsumer(sc.Name, target)
		sub := sc.intracom.Subscribe(internalServiceStates, consumer, 1)
		defer func() {
			close(checkC)
			sc.intracom.Unsubscribe(internalServiceStates, consumer)
		}()

		for {
			select {
			case <-ctx.Done():
				// signaled stop
				return
			default:
				stateBytes, ok := sub.NextMsgWithContext(ctx)
				if !ok {
					// context was cancelled before message was received.
					return
				}

				var states States
				err := json.Unmarshal(stateBytes, &states)
				if err != nil {
					sc.Log.Warnf("%s failed at unmarshalling the states for AnyServiceExitsState", sc.Name)
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
					// if we found all those we care about.
					checkC <- interestedServices
				}
			}

		}

	}()

	return checkC, cancel
}

// AnyServicesExitState
func AnyServicesExitState(sc *ServiceContext, target State, serviceNames ...string) (<-chan States, context.CancelFunc) {
	checkC := make(chan States, 1)

	ctx, cancel := context.WithCancel(sc.Ctx)

	go func() {
		// subscribe to the internal states on behalf of the service context given to _rxd.<state>.states.<service_name>
		consumer := internalExitStatesConsumer(sc.Name, target)
		sub := sc.intracom.Subscribe(internalServiceStates, consumer, 1)
		defer func() {
			close(checkC)
			sc.intracom.Unsubscribe(internalServiceStates, consumer)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				stateBytes, ok := sub.NextMsgWithContext(ctx)
				if !ok {
					// context was cancelled before message was received.
					return
				}

				var states States
				err := json.Unmarshal(stateBytes, &states)
				if err != nil {
					sc.Log.Warnf("%s failed at unmarshalling the states for AnyServiceExitsState", sc.Name)
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

// AllServicesStates
func AllServicesStates(sc *ServiceContext) (<-chan States, context.CancelFunc) {
	checkC := make(chan States, 1)

	ctx, cancel := context.WithCancel(sc.Ctx)

	go func() {
		// subscribe to the internal states on behalf of the service context given to _rxd.<state>.states.<service_name>
		consumer := internalAllStatesConsumer(sc.Name)
		sub := sc.intracom.Subscribe(internalServiceStates, consumer, 1)
		defer func() {
			close(checkC)
			sc.intracom.Unsubscribe(internalServiceStates, consumer)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				stateBytes, ok := sub.NextMsgWithContext(ctx)
				if !ok {
					// context was cancelled before message was received.
					return
				}

				var states States
				err := json.Unmarshal(stateBytes, &states)
				if err != nil {
					sc.Log.Warnf("%s failed at unmarshalling the states for AnyServiceExitsState", sc.Name)
					return
				}

				// try to send and while we wait still watch for cancel
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
