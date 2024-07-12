package rxd

import "time"

const (
	TransExitToInit StateTransition = iota
	TransInitToIdle
	TransInitToRun
	TransInitToStop
	TransInitToExit
	TransIdleToExit
	TransIdleToInit
	TransIdleToIdle
	TransIdleToRun
	TransIdleToStop
	TransRunToExit
	TransRunToInit
	TransRunToIdle
	TransRunToRun
	TransRunToStop
	TransStopToExit
	TransStopToInit
	TransStopToIdle
	TransStopToRun
	TransStopToStop
)

// StateTransition is a type that represents a state transition between two possible states the handle might transition a service to.
type StateTransition uint8

// StateTransitionTimeouts is a map of state transition to a timeout duration.
// This is used to set the timeout for a given state transition in a handler.
// If a timeout is not set for a given state transition, the handler will not
// enforce any delay between switching states.
type StateTransitionTimeouts map[StateTransition]time.Duration

func (t StateTransitionTimeouts) GetOrDefault(transition StateTransition, fallback time.Duration) time.Duration {
	if timeout, ok := t[transition]; ok {
		return timeout
	}
	return fallback
}
