package intracom

type Channel[T any] interface {
	ChannelCloser[T]
	ChannelSender[T]
	Chan() <-chan T
}

type ChannelSender[T any] interface {
	Send(message T) error
}

type ChannelCloser[T any] interface {
	Close() error
}
