package intracom

import (
	"errors"
)

type SyncBroadcaster[T any] struct{}

func (b SyncBroadcaster[T]) Broadcast(requests <-chan any, broadcast chan T) {
	subscribers := make(map[string]Channel[T])

	// nil chan so we block publishes while we have no subscribers
	var recv <-chan T
	var broadcasting bool

	for {
		select {
		case msg, ok := <-recv:
			if !ok {
				// if the publish channel is closed, then we are done
				return
			}

			for _, sub := range subscribers {
				err := sub.Send(msg)
				if err != nil {
					/// log and continue
					continue
				}
			}

		case request, open := <-requests:
			if !open {
				// if the request channel is closed, then we are done
				return
			}

			switch r := request.(type) {
			case subscribeRequest[T]:
				// handle subscribe request
				sub, exists := subscribers[r.conf.ConsumerGroup]
				if exists && r.conf.ErrIfExists {
					r.responseC <- subscribeResponse[T]{ch: sub.Chan(), err: errors.New("consumer group '" + r.conf.ConsumerGroup + "' already exists")}
					continue
				}

				if !exists {
					sub = newSubscriber[T](r.conf)
					subscribers[r.conf.ConsumerGroup] = sub
				}

				r.responseC <- subscribeResponse[T]{ch: sub.Chan(), err: nil}
				if !broadcasting && len(subscribers) > 0 {
					// enable broadcasting if we have subscribers
					recv = broadcast
					broadcasting = true
				}

			case unsubscribeRequest[T]:
				// handle unsubscribe request
				sub, exists := subscribers[r.consumer]
				if exists {
					if sub.Chan() != r.ch {
						// if the channel is not the same, then we cannot unsubscribe
						r.responseC <- unsubscribeResponse{err: errors.New("consumer group channel'" + r.consumer + "' does not match")}
						continue
					}

					delete(subscribers, r.consumer)
					err := sub.Close()
					if err != nil {
						r.responseC <- unsubscribeResponse{err: err}
						continue
					}
				}

				// TODO: we could capture sub close errors now and pass it in the unsubscribe response.
				r.responseC <- unsubscribeResponse{err: nil}

				if broadcasting && len(subscribers) == 0 {
					// disable broadcasting if we have no subscribers
					recv = nil
					broadcasting = false
				}

			case closeRequest:
				recv = nil // disable anymore publishing.
				broadcasting = false

				// handle close request
				for name, sub := range subscribers {
					delete(subscribers, name)
					err := sub.Close()
					if err != nil {
						continue
					}
				}
				// signal back that we are done
				r.responseC <- closeResponse{}
			default:
				// unknown request, do nothing.
			}
		}

	}
}
