package intracom

type Broadcaster[T any] interface {
	Broadcast(requests <-chan any, messages chan T)
}
