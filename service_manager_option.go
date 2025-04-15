package rxd

import (
	"context"
	"time"
)

type noopConfigLoader struct{}

func (noopConfigLoader) Load(ctx context.Context, contents []byte) error {
	return nil
}

type ManagerOption func(m *RunContinuousManager)

func WithInitDelay(delay time.Duration) ManagerOption {
	return func(h *RunContinuousManager) {
		h.StartupDelay = delay
	}
}

func WithTransitionTimeouts(t ManagerStateTimeouts) ManagerOption {
	return func(h *RunContinuousManager) {
		for k, v := range t {
			h.StateTimeouts[k] = v
		}
	}
}

// WithWarnDuration sets the duration after which a warning is logged
// if the service is still in a stopped state.
func WithWarnDuration(d time.Duration) ManagerOption {
	return func(h *RunContinuousManager) {
		h.WarnDuration = d
	}
}

// WithWarnDuration sets the duration after which a warning is logged
// if the service is still in a stopped state.
func WithLogWarningPolicy(policy LogWarningPolicy) ManagerOption {
	return func(h *RunContinuousManager) {
		h.LogWarningPolicy = policy
	}
}
