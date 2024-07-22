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
