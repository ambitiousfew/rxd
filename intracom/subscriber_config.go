package intracom

import "time"

type SubscriberConfig[T any] struct {
	ConsumerGroup string
	ErrIfExists   bool
	BufferSize    int
	BufferPolicy  BufferPolicyHandler[T]
	DropTimeout   time.Duration
}
