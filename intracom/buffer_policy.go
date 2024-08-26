package intracom

import (
	"errors"
	"time"
)

type BufferPolicyHandler[T any] interface {
	Handle(ch chan T, message T, stopC <-chan struct{}) error
}

type BufferPolicyDropNone[T any] struct{}

func (d BufferPolicyDropNone[T]) Handle(ch chan T, message T, stopC <-chan struct{}) error {
	select {
	case <-stopC:
		return errors.New("subscriber stopped")
	case ch <- message:
		return nil
	}
}

type BufferPolicyDropOldest[T any] struct{}

func (d BufferPolicyDropOldest[T]) Handle(ch chan T, message T, stopC <-chan struct{}) error {
	select {
	case <-stopC:
		// subscriber stopped dont try to send the message
		return errors.New("subscriber stopped")
	case ch <- message:
		// we succeeded at pushing the message
		return nil
	default:
		// we failed to push the message buffer is full
	}

	select {
	case <-stopC:
		// subscriber stopped dont try to send the message
		return errors.New("subscriber stopped")
	case <-ch:
		// dropped the oldest message
		select {
		case <-stopC:
			return errors.New("subscriber stopped")
		case ch <- message:
			// we succeeded at pushing the new message
			return nil
		default:
			// we failed to push the message buffer is still full
			return errors.New("failed to push message")
		}
	}
}

type BufferPolicyDropOldestAfterTimeout[T any] struct {
	Timer       *time.Timer
	DropTimeout time.Duration
}

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

type BufferPolicyDropNewest[T any] struct{}

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

type BufferPolicyDropNewestAfterTimeout[T any] struct {
	Timer      *time.Timer
	DropTimout time.Duration
}

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
