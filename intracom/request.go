package intracom

type subscribeRequest[T any] struct {
	conf      SubscriberConfig
	responseC chan<- subscribeResponse[T]
}

type subscribeResponse[T any] struct {
	ch  <-chan T
	err error
}

type unsubscribeRequest[T any] struct {
	consumer  string
	ch        <-chan T
	responseC chan<- unsubscribeResponse
}

type unsubscribeResponse struct {
	err error
}

type closeRequest struct {
	responseC chan<- closeResponse
}

type closeResponse struct{}
