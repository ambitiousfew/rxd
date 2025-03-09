package rxd

import (
	"strings"
)

const (
	StateExit State = iota
	StateInit
	StateIdle
	StateRun
	StateStop
)

type State uint8

func (s State) String() string {
	switch s {
	case StateInit:
		return "init"
	case StateIdle:
		return "idle"
	case StateRun:
		return "run"
	case StateStop:
		return "stop"
	case StateExit:
		return "exit"
	default:
		return "unknown"
	}
}

type ServiceStates map[string]State

func (s ServiceStates) copy() ServiceStates {
	c := make(ServiceStates, len(s))
	for k, v := range s {
		c[k] = v
	}
	return c
}

type StatesResponse struct {
	States ServiceStates
	Err    error
}

// StateUpdate reflects any given update of lifecycle state at a given time.
type StateUpdate struct {
	Name  string
	State State
}

// States is a map of service name to service state which
// reflects the service name and its lifecycle state.
type States map[string]State

// internalAllStatesConsumer returns a string that represents the internal consumer name
// this is an internal helper to help build a more unique consumer name for the internal states
// to prevent overlapping consumer group names within the same service
// format: _rxd.states.all.<consumer>
func internalAllStatesConsumer(consumer string) string {
	return strings.Join([]string{internalServiceStates, "all", consumer}, ".")
}

// internalStatesConsumer returns a string that represents the internal consumer name
// this is an internal helper to help build a more unique consumer name for the internal states
// to prevent overlapping consumer group names within the same service
// format: _rxd.states.<action>.<target>.<consumer>
func internalStatesConsumer(action ServiceAction, target State, consumer string) string {
	return strings.Join([]string{internalServiceStates, action.String(), target.String(), consumer}, ".")
}

func internalConfigConsumer(consumer string) string {
	return strings.Join([]string{internalConfigUpdate, consumer}, ".")
}
