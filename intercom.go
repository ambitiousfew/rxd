package rxd

import (
	"sync"
)

type intercom struct {
	mu          *sync.Mutex
	channels    map[string]map[string]chan []byte
	closed      bool
	lastMessage map[string][]byte
}

func NewIntercom() *intercom {
	return &intercom{
		mu:          new(sync.Mutex),
		channels:    make(map[string]map[string]chan []byte),
		closed:      false,
		lastMessage: make(map[string][]byte),
	}
}

func (i *intercom) subscribe(topic, consumer string) <-chan []byte {
	ch := make(chan []byte, 1)
	i.mu.Lock()
	defer i.mu.Unlock()

	subs, exists := i.channels[topic]
	if !exists {
		// if the topic does not yet exist, create an empty subs map for that topic.
		i.channels[topic] = make(map[string]chan []byte)
	}

	if i.lastMessage[topic] != nil {
		// if there is a previously stored message for this topic, send it upon subscribe.
		ch <- i.lastMessage[topic]
	}

	if _, exists := subs[consumer]; exists {
		// if the same consumer tries to subscribe more than once, always return the existing channel.
		return subs[consumer]
	}

	i.channels[topic][consumer] = ch
	return ch
}

func (i *intercom) unsubscribe(topic, consumer string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	subs, exists := i.channels[topic]
	if !exists {
		// cannot unsubscribe from a topic that never exists, protect against typo'd names causing nil map
		return
	}

	ch, found := subs[consumer]
	if !found {
		// cannot unsubscribe consumer if consumer never existed in the map to begin with.
		// prevent close or delete against a channel or map entry that wouldnt exist.
		return
	}
	close(ch)
	delete(i.channels[topic], consumer)
}

func (i *intercom) publish(topic string, message []byte) {
	i.mu.Lock()
	defer i.mu.Unlock()

	for _, ch := range i.channels[topic] {
		ch <- message
	}

	// store the published message as the previous message sent
	i.lastMessage[topic] = message
}

func (i *intercom) close() {
	i.mu.Lock()
	defer i.mu.Unlock()

	if !i.closed {
		for topic, subs := range i.channels {
			for _, ch := range subs {
				// close each channel for each sub
				close(ch)
			}
			// clean up the nested maps by deleting key/value
			delete(i.channels, topic)
		}

		i.closed = true
	}
}
