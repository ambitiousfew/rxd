package intracom

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type Topic[T any] interface {
	Name() string                                                           // Name returns the unique name of the topic.
	PublishChannel() chan<- T                                               // PublishChannel returns the channel publishers use to send messages to the topic.
	Subscribe(ctx context.Context, conf SubscriberConfig) (<-chan T, error) // Subscribe will attemp to add a consumer group to the topic.
	Unsubscribe(consumer string, ch <-chan T) error                         // Unsubscribe will remove the consumer group from the topic and close the subscriber channel.
	Close() error                                                           // Close will remove all consumer groups from the topic and close all channels.
}

type Subscription struct {
	Topic         string
	ConsumerGroup string
	// MaxWaitTimeout is the max time to wait before error due to a topic not existing.
	MaxWaitTimeout time.Duration
}

type TopicConfig struct {
	Name                 string // unique name for the topic
	Buffer               int    // buffer size for the topic channel
	ErrIfExists          bool   // return error if topic already exists
	SubscriberAwareCount uint32 // number of consumers to wait for before broadcasting, 0 means unaware
}

type topic[T any] struct {
	name     string
	publishC chan T
	requestC chan any
	// subscribers map[string]*subscriber[T]
	awareCount uint32 // number of consumers to wait for before broadcasting
	closed     atomic.Bool
	mu         sync.RWMutex
}

func newTopic[T any](name string, awareCount uint32, publishC chan T) Topic[T] {
	requestC := make(chan any, 1)
	t := &topic[T]{
		name:       name,
		publishC:   publishC,
		requestC:   requestC,
		awareCount: awareCount,
		// subscribers: make(map[string]*subscriber[T]),
		closed: atomic.Bool{},
		mu:     sync.RWMutex{},
	}

	// start a broadcaster for this topic
	go t.broadcast(requestC)
	return t
}

func (t *topic[T]) Name() string {
	return t.name
}

func (t *topic[T]) PublishChannel() chan<- T {
	return t.publishC
}

func (t *topic[T]) Subscribe(ctx context.Context, conf SubscriberConfig) (<-chan T, error) {
	if t.closed.Load() {
		return nil, errors.New("cannot subscribe, topic already closed")
	}

	responseC := make(chan subscribeResponse[T], 1)
	select {
	case <-ctx.Done():
		return nil, errors.New("subscribe request timed out 1")
	case t.requestC <- subscribeRequest[T]{conf: conf, responseC: responseC}:
	}

	select {
	case <-ctx.Done():
		return nil, errors.New("subscribe response timed out 2")
	case res := <-responseC:
		return res.ch, res.err
	}

}

// Unsubscribe will remove the consumer group from the topic.
func (t *topic[T]) Unsubscribe(consumer string, ch <-chan T) error {
	if t.closed.Load() {
		return errors.New("cannot unsubscribe, topic already closed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	responseC := make(chan unsubscribeResponse, 1)
	select {
	case <-ctx.Done():
		return errors.New("unsubscribe request timed out")
	case t.requestC <- unsubscribeRequest[T]{consumer: consumer, ch: ch, responseC: responseC}:
	}

	select {
	case <-ctx.Done():
		return errors.New("unsubscribe response timed out")
	case resp := <-responseC:
		return resp.err
	}

}

func (t *topic[T]) Close() error {
	if t.closed.Swap(true) {
		return errors.New("topic already closed")
	}

	responseC := make(chan closeResponse, 1)
	t.requestC <- closeRequest{responseC: responseC}

	<-responseC

	// now we can close the request channel
	close(t.requestC)
	close(t.publishC)
	return nil
}

// broadcast runs in its own routine and is responsible for broadcasting messages to all subscribers.
// It maintains a map of subscribers and listens for 3 types of requests that interrupt the broadcaster.
// 1. subscribeRequest - to add a consumer group to the topic.
// 2. unsubscribeRequest - to remove a consumer group from the topic.
// 3. closeRequest - to close the topic and all consumer groups.
//
// Parameters:
//
//	requestC <-chan any - a channel to receive requests to interrupt the broadcaster.
func (t *topic[T]) broadcast(requestC <-chan any) {
	subscribers := make(map[string]*subscriber[T])

	var publishC chan T

	var subscriberAware bool
	if t.awareCount < 1 {
		// if subscriber count is less than 1, then we are not subscriber aware
		// and we can broadcast messages to all subscribers whether they exist or not.
		publishC = t.publishC
	} else {
		// if subscriber count is greater than 0, then we are subscriber aware
		// and we need to wait for the number of subscribers to reach the count before broadcasting.
		subscriberAware = true
	}

	for {
		select {
		case msg, ok := <-publishC:
			if !ok {
				// if the publish channel is closed, then we are done
				return
			}

			var wg sync.WaitGroup
			t.mu.RLock()
			wg.Add(len(subscribers))
			for _, sub := range subscribers {
				go func() {
					sub.send(msg)
					wg.Done()
				}()
			}
			t.mu.RUnlock()

			// wait for all subscribers to finish sending the message
			wg.Wait()

		case req := <-requestC:
			switch r := req.(type) {
			case subscribeRequest[T]:
				// handle subscribe request
				sub, exists := subscribers[r.conf.ConsumerGroup]
				// sub, exists := t.subscribers[r.conf.ConsumerGroup]
				if exists && r.conf.ErrIfExists {
					r.responseC <- subscribeResponse[T]{ch: sub.ch, err: errors.New("consumer group '" + r.conf.ConsumerGroup + "' already exists")}
					continue
				}

				if !exists {
					sub = newSubscriber[T](r.conf)
					subscribers[r.conf.ConsumerGroup] = sub
				}

				r.responseC <- subscribeResponse[T]{ch: sub.ch, err: nil}

				// if we are subscriber aware, then we need to check if we can enable the publishing channel
				if subscriberAware && len(subscribers) >= int(t.awareCount) {
					publishC = t.publishC
				}

			case unsubscribeRequest[T]:
				// handle unsubscribe request
				t.mu.Lock()
				sub, exists := subscribers[r.consumer]

				if exists {
					if sub.ch != r.ch {
						// if the channel is not the same, then we cannot unsubscribe
						r.responseC <- unsubscribeResponse{err: errors.New("consumer group channel'" + r.consumer + "' does not match")}
						continue
					}

					sub.close()
					delete(subscribers, r.consumer)
				}

				// TODO: we could capture sub close errors now and pass it in the unsubscribe response.
				r.responseC <- unsubscribeResponse{err: nil}

				// if we are subscriber aware, then we need to check if we can disable the publishing channel
				if subscriberAware && len(subscribers) < int(t.awareCount) {
					publishC = nil
				}
				t.mu.Unlock()

			case closeRequest:
				publishC = nil // disable anymore publishing.

				// handle close request
				t.mu.Lock()
				for name, sub := range subscribers {
					sub.close()
					delete(subscribers, name)
				}
				t.mu.Unlock()

				// signal back that we are done
				r.responseC <- closeResponse{}
			default:
				// unknown request, do nothing.
			}
		}

	}
}
