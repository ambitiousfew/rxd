package intracom

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

type Option func(*Intracom)

// intracom struct acts as a registry for all topic channels.
type Intracom struct {
	name   string
	topics map[string]any
	mu     sync.RWMutex

	logger log.Logger
	closed atomic.Bool
}

// New creates a new instance of Intracom with the given name and logger and starts the broker routine.
func New(name string, opts ...Option) *Intracom {

	ic := &Intracom{
		name:   name,
		topics: make(map[string]any),
		logger: noopLogger{},
		closed: atomic.Bool{},
		mu:     sync.RWMutex{},
	}

	for _, opt := range opts {
		opt(ic)
	}

	return ic
}

// CreateTopic creates a new topic with the given configuration.
// Topic names must be unique, if the topic already exists, an error is returned.
func CreateTopic[T any](ic *Intracom, conf TopicConfig) (Topic[T], error) {
	if ic == nil {
		return nil, errors.New("cannot create topic '" + conf.Name + "', intracom is nil")
	}

	if ic.closed.Load() {
		return nil, errors.New("cannot create topic '" + conf.Name + "', intracom is closed")
	}

	ic.mu.RLock()
	topicAny, ok := ic.topics[conf.Name]
	ic.mu.RUnlock()
	if !ok {
		ch := make(chan T, conf.Buffer)
		topic := newTopic[T](conf.Name, ch)

		ic.mu.Lock()
		ic.topics[conf.Name] = topic
		ic.mu.Unlock()
		return topic, nil
	}

	topic, ok := topicAny.(Topic[T])
	if !ok {
		return nil, errors.New("topic already exists with a different type")
	}

	if conf.ErrIfExists {
		return topic, errors.New("topic " + conf.Name + " already exists")
	}

	return topic, nil
}

func RemoveTopic[T any](ic *Intracom, name string) error {
	if ic == nil {
		return errors.New("cannot remove topic '" + name + "', intracom is nil")
	}
	ic.mu.RLock()
	topicAny, ok := ic.topics[name]
	ic.mu.RUnlock()
	if !ok {
		return errors.New("topic " + name + " does not exist")
	}

	topic, ok := topicAny.(Topic[T])
	if !ok {
		return errors.New("topic exists but with a different type")
	}

	err := topic.Close()
	if err != nil {
		return err
	}

	ic.mu.Lock()
	delete(ic.topics, name)
	ic.mu.Unlock()
	return nil
}

// CreateSubscription will (if set) wait a max timeout for a topic to exist and the proceed to subscribe to that topic.
// If the topic does not exist within the maxWait duration, an error is returned.
// If maxWait is 0, the function will wait indefinitely for the topic to exist.
// If the intracom is closed, an error is returned.
// If the context is canceled, a context error is returned.
func CreateSubscription[T any](ctx context.Context, ic *Intracom, topic string, maxWait time.Duration, conf SubscriberConfig) (<-chan T, error) {
	if ic == nil {
		return nil, errors.New("cannot create subscription '" + topic + "<-" + conf.ConsumerGroup + "', intracom is nil")
	}

	ic.mu.RLock()
	topicAny, found := ic.topics[topic]
	ic.mu.RUnlock()

	if found {
		// if the topic exists we can subscribe to it immediately.
		t, ok := topicAny.(Topic[T])
		if !ok {
			return nil, errors.New("cannot subscribe to a topic with a different type")
		}
		return t.Subscribe(conf)
	}

	// if the topic doesn't yet exist we want to wait/poll for it to be created for maxWait duration.
	// if maxWait is 0, we wait indefinitely or until context is cancelled.
	// maxTimeout initializes to nil if maxWait is 0 so it will never trigger case <-maxTimeout.
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
			if ic.closed.Load() {
				return nil, errors.New("cannot create subscription, intracom has been closed")
			}

			ic.mu.RLock()
			topicAny, found = ic.topics[topic]
			ic.mu.RUnlock()
			// check if the topic exists yet.
			if found {
				t, exists = topicAny.(Topic[T])
				if !exists {
					return nil, errors.New("cannot subscribe to a topic with a different type")
				}
			}
		}
	}

	return t.Subscribe(conf)
}

// RemoveSubscription removes a subscription from a topic consumer name and consumer channel are required to remove the subscription.
// Normally whoever created the subscription should also be in-charge of removing it.
func RemoveSubscription[T any](ic *Intracom, topic string, consumer string, ch <-chan T) error {
	if ic == nil {
		return errors.New("cannot remove subscription '" + topic + "<-" + consumer + "', intracom is nil")
	}

	ic.mu.RLock()
	topicAny, exists := ic.topics[topic]
	ic.mu.RUnlock()

	if !exists {
		return errors.New("topic " + topic + " does not exist")
	}

	t, ok := topicAny.(Topic[T])
	if !ok {
		return errors.New("cannot unsubscribe from a topic with a different type")
	}

	return t.Unsubscribe(consumer, ch)
}

// Close interacts with the Intracom registry and closes all topics.
func Close(ic *Intracom) error {
	if ic == nil {
		return errors.New("cannot close a nil intracom")
	}

	if ic.closed.Swap(true) {
		return errors.New("intracom is already closed")
	}

	ic.mu.Lock()
	for name, topicAny := range ic.topics {
		topic, ok := topicAny.(Topic[any])
		if !ok {
			continue
		}

		err := topic.Close()
		if err != nil {
			ic.logger.Log(log.LevelError, "error closing topic", log.String("topic", name), log.Error("error", err))
		}
	}

	ic.topics = make(map[string]any)
	ic.mu.Unlock()
	return nil
}
