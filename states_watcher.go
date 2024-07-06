package rxd

import (
	"context"
	"errors"
	"log/slog"
	"os"
)

type stateUpdate struct {
	service string
	state   State
}

type stateWatcher struct {
	requestC chan interface{}
	updateC  chan stateUpdate
	log      *slog.Logger
	stopped  bool
}

type stateWatcherConfig struct {
	bufferSize int
	logger     *slog.Logger
}

func newStateWatcher(conf stateWatcherConfig) *stateWatcher {
	if conf.logger == nil {
		conf.logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})).With("rxd", "state-watcher")
	}

	return &stateWatcher{
		requestC: make(chan interface{}, 1),
		updateC:  make(chan stateUpdate, conf.bufferSize),
		log:      conf.logger, // Add your logger instance here
		stopped:  false,
	}
}

// publish is used to attempt to publish a state update of a given service.
func (sw *stateWatcher) publish(ctx context.Context, service string, state State) {
	select {
	case <-ctx.Done():
		return
	case sw.updateC <- stateUpdate{service, state}:
	}
}

func (sw *stateWatcher) subscribe(ctx context.Context, consumer string) (<-chan ServiceStates, error) {
	responseC := make(chan (<-chan ServiceStates), 1)
	defer close(responseC)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case sw.requestC <- subscribeRequest{consumer, responseC}:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-responseC:
		return resp, nil
	}

}

func (sw *stateWatcher) unsubscribe(ctx context.Context, consumer string) error {
	responseC := make(chan struct{}, 1)
	defer close(responseC)

	// send a request to unsubscribe
	select {
	case <-ctx.Done():
		return ctx.Err()
	case sw.requestC <- unsubscribeRequest{consumer, responseC}:
	}

	// NOTE: if the request was sent, now we MUST wait for the response
	// closing the response channel before request broker can reply would cause a panic.
	<-responseC
	return nil
}

func (sw *stateWatcher) watch() {
	states := make(ServiceStates)
	subscribers := make(map[string]chan ServiceStates, 0)

	for {
		select {
		case request, open := <-sw.requestC:
			if !open {
				return
			}
			switch req := request.(type) {
			case subscribeRequest:
				// subscribe to the states
				ch := make(chan ServiceStates, 1)
				subscribers[req.consumer] = ch
				req.responseC <- ch

				if len(states) > 0 {
					// if you are a late subscriber and there are states already, send the current states out.
					statesCopy := states.copy()
					sw.log.Debug("publishing states to subscriber", "subscriber", req.consumer, "states", statesCopy)
					err := send(ch, statesCopy)
					if err != nil {
						sw.log.Debug("could not publish states to subscriber", slog.Any("error", err))
					}
				}

			case unsubscribeRequest:
				// unsubscribe from the states
				s, ok := subscribers[req.consumer]
				if ok {
					close(s)
					delete(subscribers, req.consumer)
				}

				req.responseC <- struct{}{}
			}

		case update, open := <-sw.updateC:
			if !open {
				return
			}
			states[update.service] = update.state

			for name, sub := range subscribers {
				// every subscriber gets a copy of the states
				statesCopy := states.copy()
				sw.log.Debug("publishing states to subscriber", "subscriber", name, "states", statesCopy)
				err := send(sub, statesCopy)
				if err != nil {
					sw.log.Debug("could not publish states to subscriber", slog.Any("error", err))
				}
			}
		}
	}
}

func (sw *stateWatcher) stop() {
	if sw.stopped {
		return
	}
	sw.stopped = true
	close(sw.requestC)
	close(sw.updateC)
}

type subscribeRequest struct {
	consumer  string
	responseC chan (<-chan ServiceStates)
}

type unsubscribeRequest struct {
	consumer  string
	responseC chan struct{}
}

// send is a utility function that will attempt to push the current states to the subscriber, dropping oldest if channel blocks.
func send(subcriber chan ServiceStates, current ServiceStates) error {
	// atempts to send the states to the subscribers, dropping oldest if channel blocks.
	select {
	case subcriber <- current:
		return nil
	default: // couldnt push, try dropping oldest
		// try to drop oldest
		select {
		case <-subcriber:
		default:
			return errors.New("no oldest to drop")
		}

		// try to push again
		select {
		case subcriber <- current:
		default:
			return errors.New("could not send states to subscriber")
		}
	}
	return nil
}
