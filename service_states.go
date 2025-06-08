package rxd

import (
	"strings"
)

const (
	// StateUnknown is the default state of a service when it is not initialized or has no state.
	StateUnknown State = iota // 0
	// StateExit is the state when a service is exiting or has exited.
	StateExit // 1
	// StateInit is the state when a service is being initialized.
	StateInit // 2
	// StateIdle is the state when a service is idle and ready to run.
	StateIdle // 3
	// StateRun is the state when a service is running.
	StateRun // 4
	// StateStop is the state when a service is stopping or has stopped.
	StateStop // 5
)

// State respresents the given state of a service.
type State uint8

func (s State) String() string {
	switch s {
	case StateUnknown:
		return "unknown"
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

// StateUpdate is a DTO used by the states watcher to build a states map of all services.
type StateUpdate struct {
	State      State           // e.g. StateRun
	Transition StateTransition // e.g. TransitionEntering
}

// ServiceStates is a map of service names to their most recent state update.
// This is used by the states watcher to build a map of all services and their states.
type ServiceStates map[string]StateUpdate

func (s ServiceStates) copy() ServiceStates {
	c := make(ServiceStates, len(s))
	for k, v := range s {
		c[k] = v
	}
	return c
}

// StateTransition represents the transition of a service from one state to another.
type StateTransition uint8

const (
	// TransitionUnknown is the default transition when a service has no state or is not initialized.
	TransitionUnknown StateTransition = iota // 0
	// TransitionEntering is the transition when a service is entering a new state.
	TransitionEntering // 1
	// TransitionExited is the transition when a service has exited a state.
	TransitionExited // 2
)

// ServiceStateUpdate is used to signal a state change in a service.
// Used by the service manager to notify the daemon states watcher of state changes
// happening with the service it is managing.
type ServiceStateUpdate struct {
	Name       string          // e.g. "service1"
	State      State           // e.g. StateRun
	Transition StateTransition // e.g. TransitionEntering
}

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
