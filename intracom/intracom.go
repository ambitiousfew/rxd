// Package intracom provides a lightweight way to create topics and subscriptions
// used for communication between different parts of an application running concurrently.
// It allows for creating a registry of topics, subscribing to them, and removing subscriptions.
// The intracom registry, topic, subscriptions are thread-safe and can be used concurrently.
package intracom

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

// Option is a functional option for configuring the Intracom instance.
type Option func(*Intracom)

// WithLogger sets the logger for the Intracom instance.
// If the logger is nil, a noopLogger is used instead.
func WithLogger(logger log.Logger) Option {
	return func(ic *Intracom) {
		if logger != nil {
			ic.logger = logger
		} else {
			ic.logger = noopLogger{}
		}
	}
}

// Intracom acts as a registry for all topic channels.
// The Intracom struct is thread-safe and can be used concurrently.
// Use the pure generic functions below to operate against the Intracom struct:
// CreateTopic, CreateSubscription, RemoveSubscription, Close
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
		return nil, ErrTopic{Topic: conf.Name, Action: ActionCreatingTopic, Err: ErrInvalidIntracomNil}
	}

	if ic.closed.Load() {
		return nil, ErrTopic{Topic: conf.Name, Action: ActionCreatingTopic, Err: ErrIntracomClosed}
	}

	ic.mu.RLock()
	topicAny, ok := ic.topics[conf.Name]
	ic.mu.RUnlock()
	if !ok {
		topic := NewTopic[T](conf)

		ic.mu.Lock()
		ic.topics[conf.Name] = topic
		ic.mu.Unlock()
		return topic, nil
	}

	topic, ok := topicAny.(Topic[T])
	if !ok {
		return nil, ErrTopic{Topic: conf.Name, Action: ActionCreatingTopic, Err: ErrInvalidTopicType}
	}

	if conf.ErrIfExists {
		ic.logger.Log(log.LevelDebug, "could not create topic",
			log.String("name", ic.name),
			log.String("topic", conf.Name),
			log.Bool("subscriber_aware", conf.SubscriberAware),
			log.Error("error", ErrTopicAlreadyExists))
		return nil, ErrTopic{Topic: conf.Name, Action: ActionCreatingTopic, Err: ErrTopicAlreadyExists}
	}

	ic.logger.Log(log.LevelDebug, "created intracom topic",
		log.String("name", ic.name),
		log.String("topic", conf.Name),
		log.Bool("subscriber_aware", conf.SubscriberAware))

	return topic, nil
}

// RemoveTopic removes a topic from the Intracom registry.
// If the topic does not exist, an error is returned.
// If the topic is currently subscribed to, it will be closed and all subscriptions removed.
// If the topic is closed, it will be removed from the registry.
// If the Intracom is closed, an error is returned.
func RemoveTopic[T any](ic *Intracom, name string) error {
	if ic == nil {
		return ErrTopic{Topic: name, Action: ActionRemovingTopic, Err: ErrInvalidIntracomNil}
	}
	ic.mu.RLock()
	topicAny, ok := ic.topics[name]
	ic.mu.RUnlock()
	if !ok {
		return ErrTopic{Topic: name, Action: ActionRemovingTopic, Err: ErrTopicDoesNotExist}
	}

	topic, ok := topicAny.(Topic[T])
	if !ok {
		return ErrTopic{Topic: name, Action: ActionRemovingTopic, Err: ErrInvalidTopicType}
	}

	err := topic.Close()
	if err != nil {
		return ErrTopic{Topic: name, Action: ActionRemovingTopic, Err: err}
	}

	ic.mu.Lock()
	delete(ic.topics, name)
	ic.mu.Unlock()

	ic.logger.Log(log.LevelDebug, "removed intracom topic", log.String("name", ic.name), log.String("topic", name))
	return nil
}

// CreateSubscription will (if set) wait a max timeout for a topic to exist and the proceed to subscribe to that topic.
// If the topic does not exist within the maxWait duration, an error is returned.
// If maxWait is 0, the function will wait indefinitely for the topic to exist.
// If the intracom is closed, an error is returned.
// If the context is canceled, a context error is returned.
func CreateSubscription[T any](ctx context.Context, ic *Intracom, topic string, maxWait time.Duration, conf SubscriberConfig[T]) (<-chan T, error) {
	if ic == nil {
		// return nil, ErrCreatingSubscription{Topic: topic, Err: ErrInvalidIntracomNil}
		return nil, ErrSubscribe{Action: ActionCreatingSubscription, Topic: topic, Consumer: conf.ConsumerGroup, Err: ErrInvalidIntracomNil}
	}

	if ic.closed.Load() {
		// return nil, ErrCreatingSubscription{Topic: topic, Err: ErrIntracomClosed}
		return nil, ErrSubscribe{Action: ActionCreatingSubscription, Topic: topic, Consumer: conf.ConsumerGroup, Err: ErrIntracomClosed}
	}

	retryTimeout := time.NewTimer(1 * time.Nanosecond)
	defer retryTimeout.Stop()

	// if the topic doesn't yet exist we want to wait/poll for it to be created for maxWait duration.
	// if maxWait is 0, we wait indefinitely or until context is cancelled.
	// maxTimeout initializes to nil if maxWait is 0 so it will never trigger case <-maxTimeout.
	var maxTimeout <-chan time.Time
	if maxWait > 0 {
		ic.logger.Log(log.LevelDebug, "could not create subscription to topic",
			log.String("name", ic.name),
			log.String("topic", topic),
			log.String("consumer", conf.ConsumerGroup),
			log.Int("max_wait", maxWait))
		timer := time.NewTimer(maxWait)
		defer timer.Stop()
		maxTimeout = timer.C
	}

	var exists bool

	var topicAny any
	var t Topic[T]

	for !exists {
		select {
		case <-ctx.Done():
			// the caller has canceled the context, exit with error.
			return nil, ErrSubscribe{Action: ActionCreatingSubscription, Topic: topic, Consumer: conf.ConsumerGroup, Err: ctx.Err()}
		case <-maxTimeout:
			ic.logger.Log(log.LevelDebug, "could not create subscription to topic",
				log.String("name", ic.name),
				log.String("topic", topic),
				log.String("consumer", conf.ConsumerGroup),
				log.Error("error", ErrMaxTimeoutReached))
			// exceeded the callers set max timeout, exit with error.
			return nil, ErrMaxTimeoutReached
		case <-retryTimeout.C:
			if ic.closed.Load() {
				return nil, ErrSubscribe{Action: ActionCreatingSubscription, Topic: topic, Consumer: conf.ConsumerGroup, Err: ErrIntracomClosed}
			}

			ic.logger.Log(log.LevelDebug, "checking for topic existence",
				log.String("name", ic.name),
				log.String("topic", topic),
				log.String("consumer", conf.ConsumerGroup))

			// check if the topic exists yet.
			ic.mu.RLock()
			topicAny, exists = ic.topics[topic]
			ic.mu.RUnlock()
			// check again in 10ms if the topic wasnt yet found.
			retryTimeout.Reset(10 * time.Millisecond)
		}
	}

	t, exists = topicAny.(Topic[T])
	if !exists {
		return nil, ErrSubscribe{Action: ActionCreatingSubscription, Topic: topic, Consumer: conf.ConsumerGroup, Err: ErrInvalidTopicType}
	}

	return t.Subscribe(ctx, conf)
}

// RemoveSubscription removes a subscription from a topic consumer name and consumer channel are required to remove the subscription.
// Normally whoever created the subscription should also be in-charge of removing it.
func RemoveSubscription[T any](ic *Intracom, topic string, consumer string, ch <-chan T) error {
	if ic == nil {
		return ErrSubscribe{Action: ActionRemovingSubscription, Topic: topic, Consumer: consumer, Err: ErrInvalidIntracomNil}
	}

	ic.mu.RLock()
	topicAny, exists := ic.topics[topic]
	ic.mu.RUnlock()

	if !exists {
		return ErrSubscribe{Action: ActionRemovingSubscription, Topic: topic, Consumer: consumer, Err: ErrTopicNotFound}
	}

	t, ok := topicAny.(Topic[T])
	if !ok {
		return ErrSubscribe{Action: ActionRemovingSubscription, Topic: topic, Consumer: consumer, Err: ErrInvalidTopicType}
	}

	ic.logger.Log(log.LevelDebug, "removing subscription", log.String("name", ic.name), log.String("topic", topic), log.String("consumer", consumer))

	return t.Unsubscribe(consumer, ch)
}

// Close interacts with the Intracom registry and closes all topics.
// It will close all topics and remove them from the registry.
// If the Intracom is already closed, an error is returned.
// If the Intracom is nil, an error is returned.
// It is safe to call Close multiple times, it will only close the topics once.
func Close(ic *Intracom) error {
	if ic == nil {
		return ErrIntracom{Action: ActionClosingTopic, Err: ErrInvalidIntracomNil}
	}

	if ic.closed.Swap(true) {
		return ErrIntracom{Action: ActionClosingTopic, Err: ErrIntracomClosed}
	}

	ic.logger.Log(log.LevelDebug, "closing all intracom topics", log.String("name", ic.name))

	ic.mu.Lock()
	for name, topicAny := range ic.topics {
		topic, ok := topicAny.(Topic[any])
		if !ok {
			continue
		}

		ic.logger.Log(log.LevelDebug, "closing topic", log.String("name", ic.name), log.String("topic", name))
		err := topic.Close()
		if err != nil {
			ic.logger.Log(log.LevelError, "error closing topic", log.String("topic", name), log.Error("error", err))
		}
	}

	ic.topics = make(map[string]any)
	ic.mu.Unlock()
	return nil
}
