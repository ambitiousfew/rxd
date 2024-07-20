package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ambitiousfew/rxd/intracom"
)

func main() {
	parent, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ic := intracom.New("pubsub")

	someTopic := "pubsub-topic" // unique topic name
	topic, err := intracom.CreateTopic[int](ic, intracom.TopicConfig{
		Name:                 someTopic,
		ErrIfExists:          true,
		SubscriberAwareCount: 1,
	})
	if err != nil {
		log.Fatal(err)
	}

	// create a subscriber
	sub, err := topic.Subscribe(parent, intracom.SubscriberConfig{
		ConsumerGroup: "group1",
		BufferSize:    10,
		BufferPolicy:  intracom.DropNone,
	})

	if err != nil {
		intracom.Close(ic)
		log.Fatal(err)
	}

	go func(ctx context.Context, topic intracom.Topic[int]) {
		publishC := topic.PublishChannel()

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

	for msg := range sub {
		log.Println("received message", msg)
	}
	fmt.Println("done")
}
