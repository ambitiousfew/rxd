package intracom

import (
	"testing"
)

func TestIntracom_TopicSubscribe(t *testing.T) {
	testTopic, err := ic.CreateTopic(TopicConfig{
		Name:        t.Name(),
		Buffer:      1,
		ErrIfExists: true,
	})

	if err != nil {
		t.Fatalf("error creating topic: %v", err)
	}

	sub, err := testTopic.Subscribe(SubscriberConfig{
		ConsumerGroup: t.Name(),
		ErrIfExists:   true,
	})
	if err != nil {
		t.Fatalf("error subscribing to topic: %v", err)
	}

	if sub == nil {
		t.Fatalf("expected subscriber channel, got nil")
	}

}

func TestIntracom_TopicMultipleSubscribers(t *testing.T) {
	testTopic, err := ic.CreateTopic(TopicConfig{
		Name:        t.Name(),
		Buffer:      1,
		ErrIfExists: true,
	})

	if err != nil {
		t.Fatalf("error creating topic: %v", err)
	}

	sub1, err := testTopic.Subscribe(SubscriberConfig{
		ConsumerGroup: t.Name() + "_1",
		ErrIfExists:   true,
	})
	if err != nil {
		t.Fatalf("error subscribing to topic: %v", err)
	}

	sub2, err := testTopic.Subscribe(SubscriberConfig{
		ConsumerGroup: t.Name() + "_2",
		ErrIfExists:   true,
	})
	if err != nil {
		t.Fatalf("error subscribing to topic: %v", err)
	}

	if sub1 == sub2 {
		t.Fatalf("expected different subscribers, got same")
	}

}

func TestIntracom_TopicDuplicateSubscribers(t *testing.T) {
	testTopic, err := ic.CreateTopic(TopicConfig{
		Name:        t.Name(),
		Buffer:      1,
		ErrIfExists: true,
	})

	if err != nil {
		t.Fatalf("error creating topic: %v", err)
	}

	sub1, err := testTopic.Subscribe(SubscriberConfig{
		ConsumerGroup: t.Name(),
		ErrIfExists:   true,
	})

	if err != nil {
		t.Fatalf("error subscribing to topic: %v", err)
	}

	sub2, err := testTopic.Subscribe(SubscriberConfig{
		ConsumerGroup: t.Name(),
		ErrIfExists:   true,
	})

	if err == nil {
		t.Fatalf("expected error subscribing to topic, got nil")
	}

	if sub1 != sub2 {
		t.Fatalf("expected same subscribers, got different")
	}
}
