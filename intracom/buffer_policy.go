package intracom

import (
	"errors"
	"time"
)

// BufferPolicyHandler defines an interface for handling buffer policies in a channel.
// It provides a method to handle the message sending logic based on the buffer policy.
// The method takes a channel, a message of type T, and a stop channel to signal when to stop processing.
type BufferPolicyHandler[T any] interface {
	Handle(ch chan T, message T, stopC <-chan struct{}) error
}

// BufferPolicyDropNone is a buffer policy that does not drop any messages.
// It simply attempts to send the message to the channel and blocks until it can do so.
type BufferPolicyDropNone[T any] struct{}

// Handle attempts to send the message to the channel.
// If the stop channel is closed, it returns an error indicating that the subscriber has stopped.
// If the message is successfully sent, it returns nil.
func (d BufferPolicyDropNone[T]) Handle(ch chan T, message T, stopC <-chan struct{}) error {
	select {
	case <-stopC:
		return errors.New("subscriber stopped")
	case ch <- message:
		return nil
	}
}

// BufferPolicyDropOldest is a buffer policy that drops the oldest message in the channel
// when the channel is full and a new message needs to be sent.
// It attempts to send the new message, and if the channel is full, it pops the oldest message
type BufferPolicyDropOldest[T any] struct{}

// Handle attempts to send the message to the channel.
// If the stop channel is closed, it returns an error indicating that the subscriber has stopped.
// If the message is successfully sent, it returns nil.
func (d BufferPolicyDropOldest[T]) Handle(ch chan T, message T, stopC <-chan struct{}) error {
	// Always attempt to  drop first before pushing one in.
	select {
	case <-stopC:
		// subscriber stopped dont try to send the message
		return errors.New("subscriber stopped")
	case <-ch:
		// dropped 1
	default:
		// no message to drop, continue...
	}

	select {
	case <-stopC:
		// subscriber stopped dont try to send the message
		return errors.New("subscriber stopped")
	case ch <- message:
		// we succeeded at pushing the message
		return nil
	default:
		// we failed to push the message buffer is full
		return errors.New("buffer full, failed to push message")
	}
}

// BufferPolicyDropOldestAfterTimeout is a buffer policy that drops the oldest message in the channel
// after a specified timeout when the channel is full and a new message needs to be sent.
// It uses a timer to wait for the specified duration before attempting to drop the oldest message.
type BufferPolicyDropOldestAfterTimeout[T any] struct {
	Timer       *time.Timer
	DropTimeout time.Duration
}

// Handle attempts to send the message to the channel.
// If the stop channel is closed, it returns an error indicating that the subscriber has stopped.
// If the message is successfully sent, it returns nil.
func (d BufferPolicyDropOldestAfterTimeout[T]) Handle(ch chan T, message T, stopC <-chan struct{}) error {
	select {
	case <-stopC:
		// subscriber stopped dont try to send the message
		return errors.New("subscriber stopped")
	case ch <- message:
		// attempt to send to the publish channel
		return nil
	default:
		// if the channel is full, wait on timeout trying to send the message
		d.Timer.Reset(d.DropTimeout)
		select {
		case <-stopC:
			// subscriber stopped dont try to send the message
			return errors.New("subscriber stopped")
		case ch <- message:
			// we succeeded at pushing the message
			return nil
		case <-d.Timer.C:
			// timer elapsed continue... below
		}
	}

	// last attempt to send the message or we error.
	select {
	case <-stopC:
		// subscriber stopped dont try to send the message
		return errors.New("subscriber stopped")
	case <-ch: // try to pop one
		// we popped one, now try to push the message
		select {
		case <-stopC:
			// subscriber stopped dont try to send the message
			return errors.New("subscriber stopped")
		case ch <- message:
			// we succeeded at pushing the message
			return nil
		default:
			// we failed to push the message
			return errors.New("timeout exceeded, failed to push message")
		}
	}
}

// BufferPolicyDropNewest is a buffer policy that drops the newest message in the channel
// when the channel is full and a new message needs to be sent.
// It attempts to send the new message, and if the channel is full, it simply drops the new message.
type BufferPolicyDropNewest[T any] struct{}

// Handle attempts to send the message to the channel.
// If the stop channel is closed, it returns an error indicating that the subscriber has stopped.
// If the message is successfully sent, it returns nil.
// If the channel is full, it simply drops the new message without sending it.
func (d BufferPolicyDropNewest[T]) Handle(ch chan T, message T, stopC <-chan struct{}) error {
	select {
	case <-stopC:
		return errors.New("subscriber stopped")
	case ch <- message:
		// we succeeded at pushing the message
		return nil
	default:
		// we failed to push the message buffer is full
		// so just drop the current message
		return nil
	}
}

// BufferPolicyDropNewestAfterTimeout is a buffer policy that drops the newest message in the channel
// after a specified timeout when the channel is full and a new message needs to be sent.
// It uses a timer to wait for the specified duration before attempting to drop the new message.
type BufferPolicyDropNewestAfterTimeout[T any] struct {
	Timer      *time.Timer
	DropTimout time.Duration
}

// Handle attempts to send the message to the channel.
// If the stop channel is closed, it returns an error indicating that the subscriber has stopped.
// If the message is successfully sent, it returns nil.
// If the channel is full, it waits for a specified timeout before dropping the new message.
func (d BufferPolicyDropNewestAfterTimeout[T]) Handle(ch chan T, message T, stopC <-chan struct{}) error {
	select {
	case <-stopC:
		// subscriber stopped dont try to send the message
		return errors.New("subscriber stopped")
	case ch <- message:
		// attempt to send to the publish channel
		return nil
	default:
		// if the channel is full, wait on timeout trying to send the message
		d.Timer.Reset(d.DropTimout)
		select {
		case <-stopC:
			// subscriber stopped dont try to send the message
			return errors.New("subscriber stopped")
		case ch <- message:
			// we succeeded at pushing the message
			return nil
		case <-d.Timer.C:
			// timer elapsed continue... just drop the current message
			return nil
		}
	}
}
