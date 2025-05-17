package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ambitiousfew/rxd/v2/intracom"
)

func main() {
	parent, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ic := intracom.New("pubsub")

	someTopic := "pubsub-topic" // unique topic name
	topic, err := intracom.CreateTopic[int](ic, intracom.TopicConfig{
		Name:            someTopic,
		ErrIfExists:     true,
		SubscriberAware: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	// create a subscriber
	sub, err := topic.Subscribe(parent, intracom.SubscriberConfig[int]{
		ConsumerGroup: "group1",
		BufferSize:    10,
		BufferPolicy:  intracom.BufferPolicyDropNone[int]{},
	})

	if err != nil {
		intracom.Close(ic)
		log.Fatal(err)
	}

	go func(ctx context.Context, topic intracom.Topic[int]) {
		publishC := topic.PublishChannel()

		// publish 100 messages to the topic
		for i := 0; i < 100; i++ {
			select {
			case <-ctx.Done():
				topic.Close()
				return
			case publishC <- i:
			}
		}
		topic.Close()
	}(parent, topic)

	// consume the messages over subscriber channel.
	for msg := range sub {
		log.Println("received message", msg)
	}
	fmt.Println("done")
}
