package intracom

// Channel is a generic interface for a channel that can send and receive messages of type T.
type Channel[T any] interface {
	ChannelCloser[T]
	ChannelSender[T]
	Chan() <-chan T
}

// ChannelSender is a generic interface that defines a method for sending messages of type T.
type ChannelSender[T any] interface {
	Send(message T) error
}

// ChannelCloser is a generic interface that defines a method for closing the channel.
type ChannelCloser[T any] interface {
	Close() error
}
