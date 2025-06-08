package rxd

import "time"

// ManagerOption is a functional option type for configuring a RunContinuousManager.
type ManagerOption func(m *RunContinuousManager)

// WithInitDelay allows setting a custom delay before the service is initialized.
// This is useful for services that need to wait for other dependencies to be ready.
func WithInitDelay(delay time.Duration) ManagerOption {
	return func(h *RunContinuousManager) {
		h.StartupDelay = delay
	}
}

// WithTransitionTimeouts allows setting specific timeouts for state transitions.
func WithTransitionTimeouts(t ManagerStateTimeouts) ManagerOption {
	return func(h *RunContinuousManager) {
		for k, v := range t {
			h.StateTimeouts[k] = v
		}
	}
}
