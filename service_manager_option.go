package rxd

import (
	"context"
	"time"

	"github.com/ambitiousfew/rxd/config"
)

type noopConfigLoader struct{}

func (noopConfigLoader) Load(ctx context.Context, loader config.LoaderFn) error {
	return loader(ctx, map[string]any{})
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
		h.LogWarning = true
		h.WarnDuration = d
	}
}
