package intracom

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type Topic[T any] interface {
	Publisher() chan<- T
	Subscribe(conf SubscriberConfig) (<-chan T, error)
	Unsubscribe(consumer string, ch <-chan T) error
	Close() error
}

type Subscription struct {
	Topic         string
	ConsumerGroup string
	// MaxWaitTimeout is the max time to wait before error due to a topic not existing.
	MaxWaitTimeout time.Duration
}
type TopicConfig struct {
	Name        string // unique name for the topic
	Buffer      int    // buffer size for the topic channel
	ErrIfExists bool   // return error if topic already exists
}

type topic[T any] struct {
	publishC    chan T
	subscribers map[string]*subscriber[T]
	closed      atomic.Bool
	mu          sync.RWMutex
}

func newTopic[T any](publishC chan T) *topic[T] {
	t := &topic[T]{
		publishC:    publishC,
		subscribers: make(map[string]*subscriber[T]),
		closed:      atomic.Bool{},
		mu:          sync.RWMutex{},
	}

	// start a broadcaster for this topic
	go t.broadcast(publishC)
	return t
}

func (t *topic[T]) Publisher() chan<- T {
	return t.publishC
}

func (t *topic[T]) Subscribe(conf SubscriberConfig) (<-chan T, error) {
	if t.closed.Load() {
		return nil, errors.New("cannot subscribe, topic already closed")
	}

	t.mu.RLock()
	sub, ok := t.subscribers[conf.ConsumerGroup]
	if ok {
		t.mu.RUnlock()
		if conf.ErrIfExists {
			return nil, errors.New("consumer group '" + conf.ConsumerGroup + "' already exists")
		}
		return sub.ch, nil
	}
	t.mu.RUnlock()

	// subscriber did not exist so create a new one.
	sub = newSubscriber[T](conf)
	t.mu.Lock()
	t.subscribers[conf.ConsumerGroup] = sub
	t.mu.Unlock()
	return sub.ch, nil
}

func (t *topic[T]) Unsubscribe(consumer string, ch <-chan T) error {
	if t.closed.Load() {
		return errors.New("cannot unsubscribe, topic already closed")
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	sub, ok := t.subscribers[consumer]
	if !ok {
		return errors.New("consumer group '" + consumer + "' does not exist")
	}

	if sub.ch != ch {
		return errors.New("consumer group '" + consumer + "' does not match the channel provided")
	}

	sub.close()
	delete(t.subscribers, consumer)
	return nil
}

func (t *topic[T]) Close() error {
	if t.closed.Swap(true) {
		return errors.New("topic already closed")
	}

	t.mu.Lock()
	// close all subscribers first
	for name, sub := range t.subscribers {
		sub.close()
		delete(t.subscribers, name)
	}
	// now close the publish channel
	close(t.publishC)
	t.mu.Unlock()
	return nil
}

// broadcaster is a blocking function that will handle all requests to the channel.
// it will also handle broadcasting messages to all subscribers the topic its created for.
//
// NOTE: This function is only called from the broker routine.
//
// Parameters:
// - broadcastC: the channel used to receive requests to the broadcaster (subscribe, unsubscribe, close)
// - publishC: the channel used to publish messages to the broadcaster
// - doneC: the channel used to signal when the broadcaster is done for graceful shutdown
func (t *topic[T]) broadcast(publishC <-chan T) {
	for msg := range publishC {
		var wg sync.WaitGroup
		t.mu.RLock()
		for _, sub := range t.subscribers {
			wg.Add(1)
			go func() {
				sub.send(msg)
				wg.Done()
			}()
		}
		t.mu.RUnlock()

		// wait for all subscribers to finish sending the message
		wg.Wait()
	}
}
