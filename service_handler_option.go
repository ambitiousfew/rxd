package rxd

import "time"

type HandlerOption func(h *DefaultHandler)

func WithInitDelay(delay time.Duration) HandlerOption {
	return func(h *DefaultHandler) {
		h.StartupDelay = delay
	}
}

func WithTransitionTimeouts(t HandlerStateTimeouts) HandlerOption {
	return func(h *DefaultHandler) {
		for k, v := range t {
			h.StateTimeouts[k] = v
		}
	}
}
