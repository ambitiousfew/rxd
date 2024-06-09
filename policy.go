package rxd

import (
	"time"
)

const (
	// PolicyRunContinous is for long-running services that constantly restart themselves when erroring
	// these services can only be stopped by a signal or context cancelation.
	PolicyRunContinous RunPolicy = iota
	// PolicyRunOnce will only allow the a single Run to take place regardless of success/failure
	PolicyRunOnce
)

type RunPolicyConfig struct {
	Policy       RunPolicy
	RestartDelay time.Duration
}

type RunPolicy int

func (p RunPolicy) String() string {
	switch p {
	case PolicyRunContinous:
		return "run_continuous"
	case PolicyRunOnce:
		return "run_once"
	default:
		return "unknown"
	}
}
