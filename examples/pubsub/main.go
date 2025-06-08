package main

import (
	"context"
	"os"
	"strconv"
	"sync"

	"time"

	"github.com/ambitiousfew/rxd/intracom"
	"github.com/ambitiousfew/rxd/log"
)

type Measurable interface {
	Name() string
}

type Registerable interface {
	Gather(context.Context) ([]Measurable, error)
}

type metric struct {
	name string
}

func (m metric) Name() string {
	return m.name
}

type registry struct {
	metrics []Measurable
}

func (r *registry) Gather(ctx context.Context) ([]Measurable, error) {
	return r.metrics, nil
}

func main() {
	parent, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handler := log.NewHandler(log.WithWriters(os.Stdout, os.Stderr))
	logger := log.NewLogger(log.LevelDebug, handler)

	ic := intracom.New("pubsub", intracom.WithLogger(logger))

	someTopic := "pubsub-topic" // unique topic name
	topic, err := intracom.CreateTopic[Measurable](ic, intracom.TopicConfig{
		Name:            someTopic,
		ErrIfExists:     true,
		SubscriberAware: false,
	})
	if err != nil {
		logger.Log(log.LevelError, "failed to create topic", log.String("topic", someTopic), log.Error("error", err))
		os.Exit(1)
	}

	var wg sync.WaitGroup
	wg.Add(3)

	go func(ctx context.Context, topic intracom.Topic[Measurable]) {
		defer wg.Done()
		// delay publishing...
		publishC := topic.PublishChannel()
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}

		// publish 100 messages to the topic
		for i := 0; i < 100; i++ {
			select {
			case <-ctx.Done():
				topic.Close()
				return
			case publishC <- metric{name: "metric-" + strconv.Itoa(i)}:
			}
		}

		topic.Close()
	}(parent, topic)

	go func() {
		defer wg.Done()
		// create a 1st subscriber
		sub2, err := topic.Subscribe(parent, intracom.SubscriberConfig[Measurable]{
			ConsumerGroup: "group1",
			BufferSize:    10,
			BufferPolicy:  intracom.BufferPolicyDropOldest[Measurable]{},
		})

		if err != nil {
			intracom.Close(ic)
			logger.Log(log.LevelError, "failed to create subscriber", log.String("topic", someTopic), log.Error("error", err))
		}

		for msg := range sub2 {
			if msg == nil {
				logger.Log(log.LevelError, "message was nil")
				continue
			}

			logger.Log(log.LevelInfo, "received message from subscriber", log.Any("message", msg))
		}

	}()

	go func() {
		defer wg.Done()
		// create a 2nd subscriber
		sub2, err := topic.Subscribe(parent, intracom.SubscriberConfig[Measurable]{
			ConsumerGroup: "group1",
			BufferSize:    10,
			BufferPolicy:  intracom.BufferPolicyDropOldest[Measurable]{},
		})

		if err != nil {
			intracom.Close(ic)
			logger.Log(log.LevelError, "failed to create subscriber2", log.String("topic", someTopic), log.Error("error", err))
		}

		for msg := range sub2 {
			if msg == nil {
				logger.Log(log.LevelError, "message was nil")
				continue
			}

			logger.Log(log.LevelInfo, "received message from subscriber2", log.Any("message", msg))
		}

	}()

	wg.Wait()

	logger.Log(log.LevelInfo, "exiting...")
}
