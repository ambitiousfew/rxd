package intracom

import (
	"context"
	"sync"
)

type broker[T any] struct {
	publishC chan T
	channels map[string]chan T // subscriber lookup (only) channels
	mu       sync.RWMutex
}

func newBroker[T any](publishC chan T) *broker[T] {
	return &broker[T]{
		publishC: publishC,
		channels: make(map[string]chan T), // subscribers
		mu:       sync.RWMutex{},
	}
}

// get is used for looking up a specific consumer channel
func (b *broker[T]) get(ctx context.Context, consumer string) (chan T, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	ch, ok := b.channels[consumer]
	return ch, ok
}

// add is used for adding a new consumer channel
func (b *broker[T]) add(ctx context.Context, consumer string, buffer int) chan T {
	ch, found := b.get(ctx, consumer) // check if consumer already exists
	if found {
		return ch
	}

	b.mu.Lock()
	if _, ok := b.channels[consumer]; !ok {
		b.channels[consumer] = make(chan T, buffer)
	}
	return b.channels[consumer]
}
