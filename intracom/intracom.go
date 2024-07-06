package intracom

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/ambitiousfew/rxd/log"
)

type Intracom[T any] interface {
	Register(ctx context.Context, topic string) (chan<- T, func())
	Subscribe(ctx context.Context, conf SubscriberConfig) (<-chan T, func())
	Close() error
}

type Option[T any] func(*intracom[T])

// intracom is an in-memory pub/sub wrapper to enable communication between routines.
type intracom[T any] struct {
	name        string
	requestC    chan any      // channel for requests to the broker
	brokerDoneC chan struct{} // channel for broker to signal when done
	log         log.Logger

	closed  atomic.Bool
	running atomic.Bool
}

// New creates a new instance of Intracom with the given name and logger and starts the broker routine.
func New[T any](name string, logger log.Logger, opts ...Option[T]) Intracom[T] {

	ic := &intracom[T]{
		name:        name,
		log:         logger,
		requestC:    make(chan any),
		brokerDoneC: make(chan struct{}),
		closed:      atomic.Bool{},
		running:     atomic.Bool{},
	}

	go ic.broker()

	return ic
}

// Start will enable the broker to start processing requests.
// If the broker is already running, this function will return an error.
//
// NOTE: This function must be called before Register, Subscribe, or Close.
// This function is not thread-safe and should only be called once from the parent
// thread designated to manage the Intracom instance.
// func (i *intracom[T]) Start() error {
// 	if i.closed.Load() {
// 		// if intracom is already closed, return
// 		return fmt.Errorf("cannot start intracom, it is unusable once it has been closed")
// 	}

// 	if i.running.Swap(true) {
// 		// if intracom is already started, return
// 		return fmt.Errorf("cannot start intracom, it is already running")
// 	}

// 	return nil
// }

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
func (i *intracom[T]) Register(ctx context.Context, topic string) (chan<- T, func()) {
	responseC := make(chan chan T)

	i.requestC <- registerRequest[T]{
		topic:     topic,
		responseC: responseC,
	}

	publishC := <-responseC // wait for response, the publisher channel for topic given
	close(responseC)        // clean up response channel

	// return publisher channel and an unregister func reference tied to this topic registration.
	return publishC, i.unregister(ctx, topic)

}

// Subscribe will be used to subscribe to a topic. It is safe to call this function multiple times from multiple routines.
//
// * If the topic does not exist, it will be automatically registered.
// * If the consumer group already exists, the existing channel will be returned.
// * If the consumer group does not exist, it will be created.
//
// Parameters:
//   - conf: a pointer to a SubscriberConfig struct, a nil pointer will use default values (not recommended).
//     Default values:
//     -- Topic: ""
//     -- ConsumerGroup: ""
//     -- BufferSize: 1
//     -- BufferPolicy: DropNone
//
// Returns:
// - ch: the channel used to receive messages from the topic
// - unsubscribe: a function bound to this subscription that can be used to unsubscribe the consumer group
func (i *intracom[T]) Subscribe(ctx context.Context, conf SubscriberConfig) (<-chan T, func()) {
	cfg := conf
	// Buffer size should always be at least 1
	if cfg.BufferSize < 1 {
		cfg.BufferSize = 1
	}

	responseC := make(chan subscribeResponse[T])

	i.requestC <- subscribeRequest[T]{ // send subscribe request
		conf:      cfg,
		responseC: responseC,
	}

	response := <-responseC // wait for response, contains subscriber channel and success bool
	close(responseC)        // clean up response channel

	return response.ch, i.unsubscribe(ctx, cfg.Topic, cfg.ConsumerGroup)
}

// Close will shutdown the Intracom instance and all of its subscriptions.
// Intracom instance will no longer be usable after calling this function.
// This function is not safe to call from multiple routines and should only
// be called once from the parent thread designated to manage the Intracom instance.
//
// NOTE: Calling Register() or Subscribe() after calling Close will panic.
//
// Returns:
// - error: nil if successful, error if not.
func (i *intracom[T]) Close() error {
	if i.closed.Swap(true) {
		// if intracom is already closed, return
		return errors.New("cannot close intracom, already closed")
	}

	responseC := make(chan struct{}) // channel for request broker to signal when done

	i.requestC <- closeRequest{responseC: responseC} // send close request
	<-responseC                                      // wait for response signal

	close(responseC)     // clean up response channel
	close(i.brokerDoneC) // signal broker to shutdown
	close(i.requestC)    // signal broker to stop processing requests

	// i.log.Debug("intracom instance has been closed and is no longer unusable")
	return nil
}

// get performs a lookup against brokers local cache ONLY for a subscriber channel for a given topic and consumer group.
// NOTE: we do not want to interrupt the broadcaster for lookups.
//
// Parameters:
// - topic: name of the topic the subscriber is subscribed to
// - consumer: name of the consumer group containing the subscriber
//
// Returns:
// - ch: the channel used to receive messages from the topic
// - found: a boolean indicating if the subscriber channel was found
func (i *intracom[T]) get(ctx context.Context, topic, consumer string) (<-chan T, bool) {
	responseC := make(chan lookupResponse[T])
	defer close(responseC) // clean up response channel

	req := lookupRequest[T]{
		topic:     topic,
		consumer:  consumer,
		responseC: responseC,
	}
	select {
	case <-ctx.Done():
		return nil, false
	case i.requestC <- req: // send request
	}

	response := <-responseC // wait for lookup response
	return response.ch, response.found

}

// unsubscribe returns a function that can be used to unsubscribe a consumer group from a topic.
// This function will return an error if the topic does not exist or the consumer group does not exist.
// This can happen if the consumer group was never subscribed to the topic or the topic was unregistered before unsubscribing.
//
// NOTE: If this function is called, it must occur before intracom.Close() is called or it will panic sending on a closed channel.
// The expectation here is that the parent routine managing the intracom instance will call Close() after all subscribers have been
// unsubscribed. So just like Go channels, if you close a channel and then try to send on it, it will panic.
//
// Parameters:
// - topic: name of the topic the subscriber is subscribed to
// - consumer: name of the consumer group containing the subscriber
//
// Returns:
// - unsubscribe: a function bound to this subscription that can be used to unsubscribe the consumer group
func (i *intracom[T]) unsubscribe(ctx context.Context, topic, consumer string) func() {
	return func() {
		responseC := make(chan struct{})
		defer close(responseC)

		req := unsubscribeRequest[T]{ // send request
			topic:     topic,
			consumer:  consumer,
			responseC: responseC,
		}
		select {
		case <-ctx.Done():
			return
		case i.requestC <- req:
		}

		<-responseC // wait for response

	}
}

// unregister returns a function bound to this topic that can be used to unregister the topic.
// This function will return false if the topic does not exist.
func (i *intracom[T]) unregister(ctx context.Context, topic string) func() {
	return func() {
		responseC := make(chan struct{})
		defer close(responseC)

		req := unregisterRequest{ // send request to unregister topic
			topic:     topic,
			responseC: responseC,
		}
		select {
		case <-ctx.Done():
			return
		case i.requestC <- req:
		}

		<-responseC // wait for response
	}

}

// broker is a blocking function that will handle all requests to the Intracom instance.
//
// NOTE: This request broker works in-place of a mutex lock, we use a single routine to process all requests.
// We store a local cache of all topics, subscribers, and publishers to avoid interrupting the broadcaster.
//
// Each topic has its own broadcaster routine that is created when the first subscriber is registered or topic is registered.
// Only NEW subscribe/unsubscribe/register/unregister requests will interrupt the broadcaster for a given topic.
// Otherwise the request broker will use its own local cache to process requests.
func (i *intracom[T]) broker() {
	// i.log.Debug("intracom requests broker starting, accepting requests")
	// broker stores its own local cache for lookups to avoid interrupting the broadcaster.
	// yes, duplicating state but its easy enough to keep them in-sync for the benefits.

	broadcasters := make(map[string]map[chan any]chan struct{}) // broadcaster channels
	publishers := make(map[string]chan T)                       // publisher channels
	// NOTE: channels here is only for lookups, dont close channels here. its broadcasters job.
	channels := make(map[string]map[string]chan T) // subscriber lookup (only) channels

	doneC := make(chan struct{}) // channel for broadcaster to signal when done
	defer close(doneC)

	requestC := i.requestC
	var noopC chan any // noop request channel, used when broker is shutting down

	for {
		select {
		case <-i.brokerDoneC:
			// signal all broadcasters to stop and wait for them to finish
			for topic, broadcaster := range broadcasters {
				for broadcastC, doneC := range broadcaster {
					broadcastC <- closeRequest{responseC: make(chan struct{})}
					<-doneC
				}
				delete(broadcasters, topic)
			}

			// clean up channels local cache
			for topic := range channels {
				delete(channels, topic)
			}

			for topic, publishC := range publishers {
				close(publishC)
				delete(publishers, topic)
			}

			// after all broadcasters finish, clean up our local cache too.
			broadcasters = nil
			publishers = nil
			channels = nil
			// i.log.Debug("intracom requests broker is shutting down")
			return

		case noopRequest := <-noopC:
			// NOTE: broker is shutting down, we are just replying to all requests with nil/false
			switch r := noopRequest.(type) {
			case closeRequest:
				// i.log.Debug("intracom -> noop request broker", "action", "close")
				broadcasters = nil
				publishers = nil
				channels = nil
				r.responseC <- struct{}{} // ignore, reply to sender
				// i.log.Debug("intracom <- noop request broker", "action", "close", "success", false)

			case unregisterRequest:
				// i.log.Debug("intracom -> noop request broker", "action", "unregister", "topic", r.topic)
				r.responseC <- struct{}{} // ignore, reply to sender
				// i.log.Debug("intracom <- noop request broker", "action", "unregister", "topic", r.topic, "success", false)

			case registerRequest[T]:
				// i.log.Debug("intracom -> noop request broker", "action", "register", "topic", r.topic)
				r.responseC <- nil // ignore, reply to sender
				// i.log.Debug("intracom <- noop request broker", "action", "register", "topic", r.topic, "created", false)

			case lookupRequest[T]:
				// i.log.Debug("intracom -> noop request broker", "action", "lookup", "topic", r.topic, "consumer", r.consumer)
				r.responseC <- lookupResponse[T]{ch: nil, found: false} // ignore, reply to sender
				// i.log.Debug("intracom <- noop request broker", "action", "lookup", "topic", r.topic, "consumer", r.consumer, "found", false)

			case subscribeRequest[T]:
				// i.log.Debug("intracom -> noop request broker", "action", "subscribe", "topic", r.conf.Topic, "consumer", r.conf.ConsumerGroup)
				r.responseC <- subscribeResponse[T]{ch: nil, created: false} // ignore, reply to sender
				// i.log.Debug("intracom <- noop request broker", "action", "subscribe", "topic", r.conf.Topic, "consumer", r.conf.ConsumerGroup, "created", false)

			case unsubscribeRequest[T]:
				// i.log.Debug("intracom -> noop request broker", "action", "unsubscribe", "topic", r.topic, "consumer", r.consumer)
				// err := fmt.Errorf("cannot unsubscribe topic '%s' because intracom is shutting down due to context cancel", r.topic)
				r.responseC <- struct{}{} // ignore, reply to sender
				// i.log.Debug("intracom <- noop request broker", "action", "unsubscribe", "topic", r.topic, "consumer", r.consumer, "error", err)
			default:
				// fmt.Println("error: intracom noop processing unknown requests", r)
			}

		case request := <-requestC: // process requests as normal
			switch r := request.(type) {
			case closeRequest:
				// i.log.Debug("intracom -> request broker", "action", "close")
				// interrupt all broadcasters and wait for them to finish
				for topic, broadcaster := range broadcasters {
					for broadcastC, doneC := range broadcaster {
						broadcastC := broadcastC // remap for var for inline routine
						doneC := doneC           // remap for var for inline routine
						bRequest := closeRequest{responseC: make(chan struct{})}
						broadcastC <- bRequest // send close request to broadcaster
						<-bRequest.responseC   // wait for response from broadcaster
						<-doneC
					}

					delete(broadcasters, topic)
				}

				// clean up channels local cache
				for topic := range channels {
					delete(channels, topic)
				}

				// after all broadcasters finish, clean up our local cache too.
				for topic, publishC := range publishers {
					close(publishC)
					delete(publishers, topic)
				}

				noopC = i.requestC        // swap request channel to noopC
				channels = nil            // nil for gc
				publishers = nil          // prevent publishers from publishing anymore
				r.responseC <- struct{}{} // reply to sender
				// i.log.Debug("intracom <- request broker", "action", "close", "success", true)

			case unregisterRequest:
				// i.log.Debug("intracom -> request broker", "action", "unregister", "topic", r.topic)
				broadcaster, exists := broadcasters[r.topic]
				if !exists {
					// couldn't find topic, so it must not exist.
					// err := fmt.Errorf("cannot unregister topic '%s' does not exist", r.topic)
					r.responseC <- struct{}{}
					// i.log.Debug("intracom <- request broker", "action", "unregister", "topic", r.topic, "error", err)
					continue
				}

				// interrupt broadcaster for this topic
				// since we are unregistering an entire topic, we can send a close request
				for broadcastC, doneC := range broadcaster {
					// interrupt broadcaster and wait for it to finish
					bRequest := closeRequest{responseC: make(chan struct{})}
					broadcastC <- bRequest // send close request to broadcaster
					<-bRequest.responseC   // wait for response from broadcaster
					close(broadcastC)      // stop broadcaster from processing anymore published messages.
					<-doneC                // wait for broadcaster to signal complete
					close(doneC)           // close done channel
				}

				// retrieve publisher channel, close it if exists.
				publishC, exists := publishers[r.topic]
				if exists {
					close(publishC)
				}

				// cleanup topic from local cache
				delete(channels, r.topic)     // remove subscriber lookup from local cache
				delete(publishers, r.topic)   // remove publisher from local cache
				delete(broadcasters, r.topic) // remove broadcaster from local cache
				r.responseC <- struct{}{}     // reply to sender
				// i.log.Debug("intracom <- request broker", "action", "unregister", "topic", r.topic, "success", true)

			case registerRequest[T]:
				// i.log.Debug("intracom -> request broker", "action", "register", "topic", r.topic)
				// check if topic exists in local cache, if not then broadcaster routine likely hasn't been created.
				if ch, exists := publishers[r.topic]; exists {
					r.responseC <- ch // reply to sender with existing publisher channel
					// i.log.Debug("intracom <- request broker", "action", "register", "topic", r.topic, "created", false)
				} else {
					// create publisher channel
					publishC := make(chan T)
					publishers[r.topic] = publishC

					broadcastC := make(chan any) // create broadcaster channel to receive requests
					doneC := make(chan struct{}) // create done channel to signal when broadcaster is done

					// create a broadcaster request channel and done channel pair
					broadcasters[r.topic] = make(map[chan any]chan struct{})
					broadcasters[r.topic][broadcastC] = doneC

					// broadcaster will have no subscribers until a subscribe request is received.
					// publishers will be blocked from sending until first subscriber is registered.
					go i.broadcast(broadcastC, publishC, doneC) // start broadcaster for this topic

					// initialize subscriber lookup cache
					channels[r.topic] = make(map[string]chan T)
					r.responseC <- publishC // reply to sender with new publisher channel
					// i.log.Debug("intracom <- request broker", "action", "register", "topic", r.topic, "created", true)
				}

			case lookupRequest[T]:
				// i.log.Debug("intracom -> request broker", "action", "lookup", "topic", r.topic, "consumer", r.consumer)
				// check if topic exists in local cache, if not then broadcaster shouldnt have it either.
				// DO NOT interrupt broadcaster for lookup requests.
				subscribers, exists := channels[r.topic]
				if !exists {
					// topic doesnt exist, so consumer group cant exist either.
					r.responseC <- lookupResponse[T]{ch: nil, found: exists}
					// i.log.Debug("intracom <- request broker", "action", "lookup", "topic", r.topic, "consumer", r.consumer, "found", exists)
					continue
				}
				ch, found := subscribers[r.consumer]
				r.responseC <- lookupResponse[T]{ch: ch, found: found}
				// i.log.Debug("intracom <- request broker", "action", "lookup", "topic", r.topic, "consumer", r.consumer, "found", found)

			case unsubscribeRequest[T]:
				// i.log.Debug("intracom -> request broker", "action", "unsubscribe", "topic", r.topic, "consumer", r.consumer)
				// check if topic exists in local cache, if not then unsubscribing is unsuccessful.
				subscribers, exists := channels[r.topic]
				if !exists {
					// err := fmt.Errorf("cannot unsubscribe topic '%s' does not exist", r.topic) // reply to sender with false
					r.responseC <- struct{}{}
					// i.log.Debug("intracom <- request broker", "action", "unsubscribe", "topic", r.topic, "consumer", r.consumer, "error", err)
					continue
				}

				// check if consumer group exists in local cache, if not then unsubscribing is unsuccessful.
				_, found := subscribers[r.consumer]
				if !found {
					// err := fmt.Errorf("cannot unsubscribe consumer '%s' has not been subscribed", r.consumer)
					r.responseC <- struct{}{}
					// i.log.Debug("intracom <- request broker", "action", "unsubscribe", "topic", r.topic, "consumer", r.consumer, "error", err)
					continue
				}

				// consumer group exists, send unsubscribe request to broadcaster
				// var err error
				if broadcasters, exists := broadcasters[r.topic]; exists {
					// interrupt broadcaster for unsubscribe request
					for broadcastC := range broadcasters {
						// interrupt broadcaster publish to remove subscriber
						bRequest := unsubscribeRequest[T]{topic: r.topic, consumer: r.consumer, responseC: make(chan struct{})}
						broadcastC <- bRequest // send unsubscribe request to broadcaster
						<-bRequest.responseC   // wait for response from broadcaster
						close(bRequest.responseC)
					}
				}

				delete(subscribers, r.consumer) // remove subscriber from local cache
				r.responseC <- struct{}{}       // reply to sender
				// i.log.Debug("intracom <- request broker", "action", "unsubscribe", "topic", r.topic, "consumer", r.consumer, "error", err)

			case subscribeRequest[T]:
				// i.log.Debug("intracom -> request broker", "action", "subscribe", "topic", r.conf.Topic, "consumer", r.conf.ConsumerGroup)
				subscribers, exists := channels[r.conf.Topic] // check if topic exists in local cache

				if exists {
					// topic exists in local cache, a broadcaster routine should already be running for this topic.

					response := subscribeResponse[T]{ch: nil, created: false}

					if ch, found := subscribers[r.conf.ConsumerGroup]; found {
						// consumer group exists, used cached channel so we DONT interrupt broadcaster publishes.
						r.responseC <- subscribeResponse[T]{ch: ch, created: found} // reply to sender with existing channel
						// i.log.Debug("intracom <- request broker", "action", "subscribe", "topic", r.conf.Topic, "consumer", r.conf.ConsumerGroup, "created", false)
						continue
					} else {
						// consumer group DOES NOT exist, interrupt broadcaster publishes to add new subscriber channel to topic.
						responseC := make(chan subscribeResponse[T])
						subReq := subscribeRequest[T]{conf: r.conf, responseC: responseC}
						for broadcastC := range broadcasters[r.conf.Topic] {
							broadcastC <- subReq                            // send subscribe request to broadcaster
							response = <-subReq.responseC                   // update response with broadcaster response
							subscribers[r.conf.ConsumerGroup] = response.ch // update local cache
						}
						close(responseC)
					}

					r.responseC <- subscribeResponse[T]{ch: response.ch, created: response.created} // reply to sender
					// i.log.Debug("intracom <- request broker", "action", "subscribe", "topic", r.conf.Topic, "consumer", r.conf.ConsumerGroup, "created", response.created)
					continue // continue to next request
				}

				// topic does not exist in local cache, so it must not exist in broadcaster cache either.
				//   we will perform the same steps as a topic register and then subscribe the consumer to it.
				//   this way if a register comes in after, it will only cost us a local cache lookup and we
				//   avoid interrupting the broadcaster.

				publishC := make(chan T)            // create new publisher channel
				publishers[r.conf.Topic] = publishC // update local cache

				broadcastC := make(chan any) // create a new broadcaster channel to receive requests
				doneC := make(chan struct{}) // create a new signal done channel for the new broadcaster

				// each broadcaster topic map only contains 1 entry as a channel pair:
				//   key: broadcaster request channel used for broadcaster to receive requests (subscribe, unsubscribe, close) requests
				//   value: broadcaster done channel used to signal when broadcaster is done (exiting)
				broadcasters[r.conf.Topic] = make(map[chan any]chan struct{}) // create new map to hold broadcaster channels
				broadcasters[r.conf.Topic][broadcastC] = doneC                // add broadcaster channels to map

				go i.broadcast(broadcastC, publishC, doneC) // launch a new broadcaster routine for this topic

				responseC := make(chan subscribeResponse[T])
				broadcastC <- subscribeRequest[T]{conf: r.conf, responseC: responseC} // send subscribe request IMMEDIATELY to new broadcaster
				response := <-responseC                                               // wait for response from broadcaster
				close(responseC)

				channels[r.conf.Topic] = make(map[string]chan T)           // initialize subscriber lookup map
				channels[r.conf.Topic][r.conf.ConsumerGroup] = response.ch // update local cache with the subscriber channel broadcaster created
				r.responseC <- response                                    // reply to sender by forwarding broadcaster response which contains the new subscriber channel

				// i.log.Debug("intracom <- request broker", "action", "subscribe", "topic", r.conf.Topic, "consumer", r.conf.ConsumerGroup, "created", true)
			}
		}
	}
}

// broadcaster is a blocking function that will handle all requests to the channel.
// it will also handle broadcasting messages to all subscribers the topic its created for.
//
// NOTE: This function is only called from the broker routine.
//
// Parameters:
// - broadcastC: the channel used to receive requests to the broadcaster (subscribe, unsubscribe, close)
// - publishC: the channel used to publish messages to the broadcaster
// - doneC: the channel used to signal when the broadcaster is done for graceful shutdown
func (i *intracom[T]) broadcast(broadcastC <-chan any, publishC <-chan T, doneC chan<- struct{}) {
	// i.log.Debug("intracom broadcaster started and accepting requests")
	subscribers := make(map[string]*subscriber[T])

	var lastMsg T
	var receivedOnce bool

	// publish := publishC
	var publish <-chan T // dont enable until we have at least 1 subscriber.

	for {
		select {
		case request := <-broadcastC:
			// if a request comes in during a broadcast, selecting this case will
			// pause publishing long enough to update subscriber map
			// then continue publishing.
			switch r := request.(type) {
			case closeRequest:
				// close all subscriber channels.
				for _, subscriber := range subscribers {
					subscriber.close()
				}
				subscribers = nil         // nil for gc
				publish = nil             // ensure we dont receive published messages anymore
				broadcastC = nil          // ensure we dont receive any more requests
				r.responseC <- struct{}{} // reply to sender
				doneC <- struct{}{}       // signal to broker that we are done
				// i.log.Debug("intracom broadcaster closed, no longer accepting requests")
				return

			case unsubscribeRequest[T]:
				// attempt to remove from subscriber map so the publisher is able to
				//  detach before requester cancels the consumer channel.
				subscriber, found := subscribers[r.consumer]
				if !found {
					// i.log.Debug(fmt.Sprintf("cannot unsubscribe consumer '%s' has not been subscribed", r.consumer))
					r.responseC <- struct{}{}
					continue
				}

				subscriber.close()              // close subscriber channel
				delete(subscribers, r.consumer) // remove subscriber from local cache
				r.responseC <- struct{}{}

			case subscribeRequest[T]:
				if subscriber, exists := subscribers[r.conf.ConsumerGroup]; !exists {
					// consumer group doesnt exist, create new one.
					// subscriberC := make(chan T, r.conf.BufferSize)
					s := newSubscriber[T](r.conf)
					subscribers[r.conf.ConsumerGroup] = s
					r.responseC <- subscribeResponse[T]{ch: s.ch, created: true} // reply to sender

					// if we have received at least one published message, we have a lastMsg to send to new subscriber
					if receivedOnce {
						s.send(lastMsg) // send last message published to this topic to new subscriber
					}

					if len(subscribers) == 1 {
						publish = publishC // we have at least 1 subscriber, enable publishing
					}

				} else {
					// consumer group exists, pass back existing channel reference.
					r.responseC <- subscribeResponse[T]{ch: subscriber.ch, created: false} // reply to sender
				}

			}

		// NOTE: Anytime select chooses broadcastC, it interrupts the publishing.
		// So ideally we only want to send requests to broadcastC if they are necessary, such as:
		// the creation or deletion of a subscriber because we need to update the subscribers map
		// between publishing.
		case msg := <-publish:
			if !receivedOnce {
				receivedOnce = true
			}

			for _, subscriber := range subscribers {
				subscriber.send(msg)
				// sub <- msg // send message to all subscribers
			}
			lastMsg = msg // store last message broadcasted
		}

	}
}
