package intracom

import (
	"errors"
	"sync/atomic"
	"time"
)

type subscriber[T any] struct {
	consumerGroup string
	bufferSize    int
	bufferPolicy  BufferPolicyHandler[T]
	dropTimeout   time.Duration
	ch            chan T
	stopC         chan struct{}
	closed        *atomic.Bool
}

func newSubscriber[T any](conf SubscriberConfig[T]) subscriber[T] {
	var bufferPolicy BufferPolicyHandler[T]
	// ensure timer is set for timeout buffer policies
	switch bp := conf.BufferPolicy.(type) {
	case BufferPolicyDropOldestAfterTimeout[T]:
		if bp.Timer == nil {
			bp.Timer = time.NewTimer(conf.DropTimeout)
		}
		bp.Timer.Stop()
	case BufferPolicyDropNewestAfterTimeout[T]:
		if bp.Timer == nil {
			bp.Timer = time.NewTimer(conf.DropTimeout)
		}
		bp.Timer.Stop()
		bufferPolicy = bp
	case nil:
		// if no buffer policy is set, use the default one
		bufferPolicy = BufferPolicyDropNone[T]{}
	default:
		bufferPolicy = bp
	}

	return subscriber[T]{
		consumerGroup: conf.ConsumerGroup,
		bufferSize:    conf.BufferSize,
		bufferPolicy:  bufferPolicy,
		dropTimeout:   conf.DropTimeout,
		ch:            make(chan T, conf.BufferSize),
		stopC:         make(chan struct{}),
		closed:        &atomic.Bool{},
	}
}

func (s subscriber[T]) Chan() <-chan T {
	return s.ch
}

// send sends a message to the subscriber's channel.
// if the channel is full, the buffer policy will come into effect on
// how to handle the message.
func (s subscriber[T]) Send(message T) error {
	if s.closed.Load() {
		return errors.New("subscriber already closed")
	}

	return s.bufferPolicy.Handle(s.ch, message, s.stopC)
}

func (s subscriber[T]) Close() error {
	if s.closed.Swap(true) {
		return errors.New("subscriber already closed")
	}

	//signal to stop
	close(s.stopC)

	// stop the timer if it was a timeout buffer policy
	switch bp := s.bufferPolicy.(type) {
	case BufferPolicyDropOldestAfterTimeout[T]:
		bp.Timer.Stop()
	case BufferPolicyDropNewestAfterTimeout[T]:
		bp.Timer.Stop()
	}

	close(s.ch)
	return nil
}
