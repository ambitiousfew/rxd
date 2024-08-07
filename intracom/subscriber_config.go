package intracom

import "time"

// type BufferPolicy int

// const (
// 	DropNone BufferPolicy = iota
// 	DropOldest
// 	DropOldestAfterTimeout
// 	DropNewest
// 	DropNewestAfterTimeout
// )

type SubscriberConfig[T any] struct {
	ConsumerGroup string
	ErrIfExists   bool
	BufferSize    int
	BufferPolicy  BufferPolicyHandler[T]
	DropTimeout   time.Duration
}
