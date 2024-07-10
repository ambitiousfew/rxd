package intracom

import "time"

type BufferPolicy int

const (
	DropNone BufferPolicy = iota
	DropOldest
	DropOldestAfterTimeout
	DropNewest
	DropNewestAfterTimeout
)

type SubscriberConfig struct {
	ConsumerGroup string
	ErrIfExists   bool
	BufferSize    int
	BufferPolicy  BufferPolicy
	DropTimeout   time.Duration
}
