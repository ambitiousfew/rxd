package intracom

// Broadcaster is a generic interface for broadcasting messages of type T.
// It defines a method Broadcast that takes a channel of requests and a channel for messages.
type Broadcaster[T any] interface {
	Broadcast(requests <-chan any, messages chan T)
}
