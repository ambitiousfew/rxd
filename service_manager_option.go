package rxd

import "time"

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
		h.LogWarning = true
		h.WarnDuration = d
	}
}
