package rxd

import "strings"

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
	_prefix           string = "_rxd"
	internalStates    string = _prefix + ".states"
	internalAllStates string = internalStates + ".all"
)

func internalStatesConsumer(action ServiceAction, target ServiceState, consumer string) string {
	// consumer name: _rxd.states.entering.<target_state>.<consumer_group>
	return strings.Join([]string{internalStates, action.String(), target.String(), consumer}, ".")
}
