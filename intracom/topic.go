package intracom

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type Topic[T any] interface {
	Name() string
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
	name        string
	publishC    chan T
	subscribers map[string]*subscriber[T]
	closed      atomic.Bool
	mu          sync.RWMutex
}

func newTopic[T any](name string, publishC chan T) Topic[T] {
	t := &topic[T]{
		name:        name,
		publishC:    publishC,
		subscribers: make(map[string]*subscriber[T]),
		closed:      atomic.Bool{},
		mu:          sync.RWMutex{},
	}

	// start a broadcaster for this topic
	go t.broadcast(publishC)
	return t
}

func (t *topic[T]) Name() string {
	return t.name
}

func (t *topic[T]) Publisher() chan<- T {
	return t.publishC
}

func (t *topic[T]) Subscribe(conf SubscriberConfig) (<-chan T, error) {
	if t.closed.Load() {
		return nil, errors.New("cannot subscribe, topic already closed")
	}

	sub, exists := t.get(conf.ConsumerGroup)
	if !exists {
		t.mu.Lock()
		sub = newSubscriber[T](conf)
		t.subscribers[conf.ConsumerGroup] = sub
		t.mu.Unlock()
		return sub.ch, nil
	}

	if conf.ErrIfExists {
		return sub.ch, errors.New("consumer group '" + conf.ConsumerGroup + "' already exists")
	}

	return sub.ch, nil
}

func (t *topic[T]) Unsubscribe(consumer string, ch <-chan T) error {
	if t.closed.Load() {
		return errors.New("cannot unsubscribe, topic already closed")
	}

	sub, exists := t.get(consumer)
	if !exists {
		return errors.New("consumer group '" + consumer + "' does not exist")
	}

	if sub.ch != ch {
		return errors.New("consumer group '" + consumer + "' does not match the channel provided")
	}

	t.mu.Lock()
	sub.close()
	delete(t.subscribers, consumer)
	t.mu.Unlock()

	return nil
}

func (t *topic[T]) get(consumer string) (*subscriber[T], bool) {
	t.mu.RLock()
	sub, ok := t.subscribers[consumer]
	t.mu.RUnlock()
	return sub, ok
}

func (t *topic[T]) Close() error {
	if t.closed.Swap(true) {
		return errors.New("topic already closed")
	}

	t.mu.Lock()
	for name, sub := range t.subscribers {
		sub.close()
		delete(t.subscribers, name)
	}
	t.mu.Unlock()

	// now close the publish channel
	close(t.publishC)
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
		wg.Add(len(t.subscribers))
		for _, sub := range t.subscribers {
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
