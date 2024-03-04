package rxd

import (
	"fmt"
)

type stateUpdate struct {
	service string
	state   ServiceState
}

type ServiceStates map[string]ServiceState

func (s ServiceStates) copy() ServiceStates {
	c := make(ServiceStates, len(s))
	for k, v := range s {
		c[k] = v
	}
	return c
}

// ServiceState represents the possible states a service can be in.
type ServiceState int

func (s ServiceState) String() string {
	switch s {
	case unknown:
		return "unknown"
	case Init:
		return "init"
	case Idle:
		return "idle"
	case Run:
		return "run"
	case Stop:
		return "stop"
	case Exit:
		return "exit"
	default:
		return "unknown"
	}
}

const (
	// unexported to prevent user selecting states that aren't lifecycles.
	unknown ServiceState = iota
	Init
	Idle
	Run
	Stop
	Exit
)

const (
	_prefix             string = "_rxd"
	internalAllStates   string = _prefix + ".all.states"
	internalExitStates  string = _prefix + ".enter.states"
	internalEnterStates string = _prefix + ".exit.states"
)

func internalAllStatesConsumer(name string) string {
	// consumer name: _rxd.all.states.<consumer_group>
	return fmt.Sprintf("%s.%s", internalAllStates, name)
}

func internalEnterStatesConsumer(name string, state ServiceState) string {
	// consumer name: _rxd.enter.states.<target_state>.<consumer_group>
	return fmt.Sprintf("%s.%s.%s", internalEnterStates, state, name)
}

func internalExitStatesConsumer(name string, state ServiceState) string {
	// consumer name: _rxd.exit.states.<target_state>.<consumer_group>
	return fmt.Sprintf("%s.%s.%s", internalExitStates, state, name)
}
