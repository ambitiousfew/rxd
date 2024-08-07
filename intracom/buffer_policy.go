package intracom

import (
	"errors"
	"time"
)

type BufferPolicyHandler[T any] interface {
	Handle(ch chan T, message T, stopC <-chan struct{}) error
}

type DropNoneHandler[T any] struct{}

func (d DropNoneHandler[T]) Handle(ch chan T, message T, stopC <-chan struct{}) error {
	select {
	case <-stopC:
		return errors.New("subscriber stopped")
	case ch <- message:
		return nil
	}
}

type DropOldestHandler[T any] struct{}

func (d DropOldestHandler[T]) Handle(ch chan T, message T, stopC <-chan struct{}) error {
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

type DropOldestAfterTimeoutHandler[T any] struct {
	Timer       *time.Timer
	DropTimeout time.Duration
}

func (d DropOldestAfterTimeoutHandler[T]) Handle(ch chan T, message T, stopC <-chan struct{}) error {
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

type DropNewestHandler[T any] struct{}

func (d DropNewestHandler[T]) Handle(ch chan T, message T, stopC <-chan struct{}) error {
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

type DropNewestAfterTimeoutHandler[T any] struct {
	Timer      *time.Timer
	DropTimout time.Duration
}

func (d DropNewestAfterTimeoutHandler[T]) Handle(ch chan T, message T, stopC <-chan struct{}) error {
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
