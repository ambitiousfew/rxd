package intracom

import (
	"context"
	"errors"
	"sync"
)

type Topic[T any] interface {
	Subscribe(ctx context.Context, conf SubscriberConfig) (<-chan T, error)
	Unsubscribe(ctx context.Context, consumerGroup string) error
}

type TopicConfig struct {
	Topic       string
	ErrIfExists bool
}

type topic[T any] struct {
	publishC    chan T
	subscribers map[string]chan T
	mu          sync.RWMutex
}

func NewTopic[T any](publishC chan T) Topic[T] {
	return &topic[T]{
		publishC:    publishC,
		subscribers: make(map[string]chan T),
		mu:          sync.RWMutex{},
	}
}

func (t *topic[T]) Subscribe(ctx context.Context, conf SubscriberConfig) (<-chan T, error) {
	t.mu.RLock()
	ch, ok := t.subscribers[conf.ConsumerGroup]
	t.mu.RUnlock()
	if ok {
		// subscriber already exists
		if conf.ErrIfExists {
			// return error if subscriber already exists
			return nil, errors.New("consumer group '" + conf.ConsumerGroup + " already exists")
		}
		// subscriber already exists so return the channel
		return ch, nil
	}

	// subscriber did not exist so create a new one.
	ch = make(chan T, conf.BufferSize)
	t.mu.Lock()
	t.subscribers[conf.ConsumerGroup] = ch
	t.mu.Unlock()
	return ch, nil
}

func (t *topic[T]) Unsubscribe(ctx context.Context, consumerGroup string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	ch, ok := t.subscribers[consumerGroup]
	if !ok {
		return errors.New("consumer group '" + consumerGroup + "' does not exist")
	}
	close(ch)
	delete(t.subscribers, consumerGroup)
	return nil
}
