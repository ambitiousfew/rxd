package intracom

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/ambitiousfew/rxd/log"
)

type Intracom[T any] interface {
	Register(topic string, ch <-chan T) (Topic[T], error)

	Close() error
}

type Option[T any] func(*intracom[T])

// intracom is an in-memory pub/sub wrapper to enable communication between routines.
type intracom[T any] struct {
	name    string
	closed  atomic.Bool
	mu      sync.RWMutex
	topics  map[string]*topic[T]
	logger  log.Logger
	running atomic.Bool
}

// New creates a new instance of Intracom with the given name and logger and starts the broker routine.
func New[T any](name string, opts ...Option[T]) Intracom[T] {

	ic := &intracom[T]{
		name:    name,
		topics:  make(map[string]*topic[T]),
		logger:  noopLogger{},
		closed:  atomic.Bool{},
		running: atomic.Bool{},
		mu:      sync.RWMutex{},
	}

	for _, opt := range opts {
		opt(ic)
	}

	return ic
}

// Register will register a topic with the Intracom instance.
// It is safe to call this function multiple times for the same topic.
// If the topic already exists, this function will return the existing publisher channel.
//
// Parameters:
// - topic: name of the topic to register
//
// Returns:
// - publishC: the channel used to publish messages to the topic
// - unregister: a function bound to this topic that can be used to unregister the topic
func (i *intracom[T]) Register(topic string, ch <-chan T) (Topic[T], error) {
	if i.closed.Load() {
		return nil, errors.New("cannot register topic, intracom is closed")
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	t, ok := i.topics[topic]
	if ok {
		return t, errors.New("topic " + topic + " already exists")
	}

	t = newTopic[T](ch)
	i.topics[topic] = t

	return t, nil
}

func (i *intracom[T]) Close() error {
	if i.closed.Swap(true) {
		// if intracom is already closed, return
		return errors.New("cannot close intracom, already closed")
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	for name, topic := range i.topics {
		err := topic.Close()
		if err != nil {
			return err
		}
		delete(i.topics, name)
	}
	return nil
}
