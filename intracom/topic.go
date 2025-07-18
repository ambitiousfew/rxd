package intracom

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Topic is an interface that defines the behavior for a topic in the intracom system.
type Topic[T any] interface {
	Name() string                                                              // Name returns the unique name of the topic.
	PublishChannel() chan<- T                                                  // PublishChannel returns the channel publishers use to send messages to the topic.
	Subscribe(ctx context.Context, conf SubscriberConfig[T]) (<-chan T, error) // Subscribe will attemp to add a consumer group to the topic.
	Unsubscribe(consumer string, ch <-chan T) error                            // Unsubscribe will remove the consumer group from the topic and close the subscriber channel.
	Close() error                                                              // Close will remove all consumer groups from the topic and close all channels.
}

// TopicOption is a functional option type for configuring topics.
// It allows for setting various properties of a topic, such as the broadcaster.
type TopicOption[T any] func(*topic[T])

// WithBroadcaster sets the broadcaster for the topic.
// This allows for custom broadcasting behavior, such as using a specific implementation of Broadcaster.
func WithBroadcaster[T any](b Broadcaster[T]) TopicOption[T] {
	return func(t *topic[T]) {
		t.bc = b
	}
}

// Subscription is a struct that defines a subscription to a topic.
// It includes the topic name, consumer group, and a max wait timeout for topic existence checks.
type Subscription struct {
	Topic         string
	ConsumerGroup string
	// MaxWaitTimeout is the max time to wait before error due to a topic not existing.
	MaxWaitTimeout time.Duration
}

// TopicConfig defines the configuration for a topic in the intracom system.
// It includes the topic name, whether to return an error if the topic already exists,
type TopicConfig struct {
	Name            string // unique name for the topic
	ErrIfExists     bool   // return error if topic already exists
	SubscriberAware bool   // if true, topic broadcaster wont broadcast if there are no subscribers.
}

// NewTopic is a generic function that creates a new topic with the given configuration and options.
// It initializes the topic with a publish channel and a request channel for subscription management.
func NewTopic[T any](conf TopicConfig, opts ...TopicOption[T]) Topic[T] {
	publishC := make(chan T)
	requestC := make(chan any, 1)

	t := &topic[T]{
		name:     conf.Name,
		publishC: publishC,
		requestC: requestC,
		closed:   atomic.Bool{},
		bc: SyncBroadcaster[T]{
			SubscriberAware: conf.SubscriberAware,
		},
		mu: sync.RWMutex{},
	}

	for _, opt := range opts {
		opt(t)
	}

	// start a broadcaster for this topic
	go t.bc.Broadcast(requestC, publishC)

	return t
}

type topic[T any] struct {
	name     string
	publishC chan T
	requestC chan any
	bc       Broadcaster[T]
	closed   atomic.Bool
	mu       sync.RWMutex
}

func (t *topic[T]) Name() string {
	return t.name
}

func (t *topic[T]) PublishChannel() chan<- T {
	return t.publishC
}

func (t *topic[T]) Subscribe(ctx context.Context, conf SubscriberConfig[T]) (<-chan T, error) {
	if t.closed.Load() {
		return nil, errors.New("cannot subscribe, topic already closed")
	}

	responseC := make(chan subscribeResponse[T], 1)
	select {
	case <-ctx.Done():
		return nil, errors.New("subscribe request cancelled while requesting")
	case t.requestC <- subscribeRequest[T]{conf: conf, responseC: responseC}:
	}

	select {
	case <-ctx.Done():
		return nil, errors.New("subscribe response cancelled while waiting for response")
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
