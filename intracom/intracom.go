package intracom

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

type Intracom[T any] interface {
	// CreateTopic creates a new topic with the given configuration. If the topic already exists, an error is returned.
	CreateTopic(conf TopicConfig) (Topic[T], error)
	// RemoveTopic removes a topic from the intracom. If the topic does not exist, an error is returned.
	RemoveTopic(topic Topic[T]) error
	// CreateSubscription subscribes (using max wait) to a topic and returns that topic back once that topic exists.
	CreateSubscription(ctx context.Context, topic string, maxTimeout time.Duration, conf SubscriberConfig) (<-chan T, error)
	// RemoveSubscription removes a subscription from a topic consumer name and channel are required to remove the subscription.
	RemoveSubscription(topic string, consumer string, ch <-chan T) error
	Close() error
}

type Option[T any] func(*intracom[T])

// intracom is an in-memory pub/sub wrapper to enable communication between routines.
type intracom[T any] struct {
	name   string
	topics map[string]*topic[T]
	logger log.Logger

	closed  atomic.Bool
	running atomic.Bool
	mu      sync.RWMutex
}

// New creates a new instance of Intracom with the given name and logger and starts the broker routine.
func New[T any](name string, opts ...Option[T]) Intracom[T] {

	ic := &intracom[T]{
		name:    name,
		topics:  make(map[string]*topic[T]),
		logger:  noopLogger{},
		closed:  atomic.Bool{},
		running: atomic.Bool{},
		mu:      sync.RWMutex{},
	}

	for _, opt := range opts {
		opt(ic)
	}

	return ic
}

// CreateTopic creates a new topic with the given configuration.
// Topic names must be unique, if the topic already exists, an error is returned.
func (i *intracom[T]) CreateTopic(conf TopicConfig) (Topic[T], error) {
	if i.closed.Load() {
		return nil, errors.New("cannot register topic, intracom is closed")
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	t, ok := i.topics[conf.Name]
	if ok {
		return t, errors.New("topic " + conf.Name + " already exists")
	}

	ch := make(chan T, conf.Buffer)
	t = newTopic[T](ch)
	i.topics[conf.Name] = t

	return t, nil
}

func (i *intracom[T]) RemoveTopic(topic Topic[T]) error {
	if i.closed.Load() {
		return errors.New("cannot remove topic, intracom is closed")
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	for name, t := range i.topics {
		if t == topic {
			err := t.Close()
			if err != nil {
				i.logger.Log(log.LevelError, "error closing topic", log.String("topic", name), log.Error("error", err))
			}
			delete(i.topics, name)
			return nil
		}
	}

	return errors.New("topic not found")
}

// CreateSubscription will (if set) wait a max timeout for a topic to exist and the proceed to subscribe to that topic.
// If the topic does not exist within the maxWait duration, an error is returned.
// If maxWait is 0, the function will wait indefinitely for the topic to exist.
// If the intracom is closed, an error is returned.
// If the context is canceled, a context error is returned.
func (i *intracom[T]) CreateSubscription(ctx context.Context, topic string, maxWait time.Duration, conf SubscriberConfig) (<-chan T, error) {
	if i.closed.Load() {
		return nil, errors.New("cannot create subscription, intracom is closed")
	}

	var maxTimeout <-chan time.Time
	if maxWait > 0 {
		timer := time.NewTimer(maxWait)
		defer timer.Stop()
		maxTimeout = timer.C
	}

	topicCheckTimeout := time.NewTicker(100 * time.Millisecond)
	defer topicCheckTimeout.Stop()

	var exists bool
	var t Topic[T]

	for !exists {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-maxTimeout:
			return nil, errors.New("subscription max timeout reached")
		case <-topicCheckTimeout.C:
			// ensure intracom is still open
			if i.closed.Load() {
				return nil, errors.New("cannot create subscription, intracom has been closed")
			}
			// check if the topic exists yet.
			i.mu.RLock()
			t, exists = i.topics[topic]
			i.mu.RUnlock()
		}
	}

	return t.Subscribe(conf)

}

// RemoveSubscription removes a subscription from a topic consumer name and consumer channel are required to remove the subscription.
// Normally whoever created the subscription should also be in-charge of removing it.
func (i *intracom[T]) RemoveSubscription(topic string, consumer string, ch <-chan T) error {
	if i.closed.Load() {
		return errors.New("cannot remove subscription, intracom is closed")
	}

	i.mu.RLock()
	t, ok := i.topics[topic]
	i.mu.RUnlock()
	if !ok {
		return errors.New("topic " + topic + " does not exist")
	}

	err := t.Unsubscribe(consumer, ch)
	if err != nil {
		return err
	}

	return nil
}

func (i *intracom[T]) Close() error {
	if i.closed.Swap(true) {
		// if intracom is already closed, return
		return errors.New("cannot close intracom, already closed")
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	for name, topic := range i.topics {
		err := topic.Close()
		if err != nil {
			i.logger.Log(log.LevelError, "error closing topic", log.String("topic", name), log.Error("error", err))
		}
		delete(i.topics, name)
	}
	return nil
}
