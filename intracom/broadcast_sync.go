package intracom

import (
	"errors"
)

type SyncBroadcaster[T any] struct {
	SubscriberAware bool // if true, broadcaster wont broadcast if there are no subscribers.
}

func (b SyncBroadcaster[T]) Broadcast(requests <-chan any, broadcast chan T) {
	subscribers := make(map[string]Channel[T])

	var recv <-chan T     // initialized to a blocking channel
	var broadcasting bool // initialized to false

	if !b.SubscriberAware {
		// if we are not subscriber aware, then we do non-blocking broadcast regardless of subscribers.
		// this means publishing before subscribing is allowed and subscribers can miss messages.
		recv = broadcast
		broadcasting = true
	}

	var lastMessage T
	var haveReceived bool
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

			// store the previous broadcasted message.
			lastMessage = msg
			haveReceived = true

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
					newSub := newSubscriber(r.conf)
					subscribers[r.conf.ConsumerGroup] = newSub
					// if you are a new subscriber, then we try to send the last message of topic.

					if haveReceived {
						select {
						case newSub.ch <- lastMessage:
						default:
							// if the channel is full or unbuffered, then we dont send last message.
						}
					}
					r.responseC <- subscribeResponse[T]{ch: newSub.ch, err: nil}
				} else {
					r.responseC <- subscribeResponse[T]{ch: sub.Chan(), err: nil}
				}

				if b.SubscriberAware && !broadcasting && len(subscribers) > 0 {
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

				if b.SubscriberAware && broadcasting && len(subscribers) < 1 {
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
