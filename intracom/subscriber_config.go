package intracom

import "time"

// SubscriberConfig defines the configuration for a subscriber in the system.
// It includes the consumer group name, buffer size, buffer policy handler,
// and drop timeout. The ErrIfExists flag indicates whether an error should be returned
// if a subscription with the same consumer group already exists.
type SubscriberConfig[T any] struct {
	ConsumerGroup string
	ErrIfExists   bool
	BufferSize    int
	BufferPolicy  BufferPolicyHandler[T]
	DropTimeout   time.Duration
}
