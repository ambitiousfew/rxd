package intracom

import (
	"context"
	"sync"
	"testing"
	"time"
)

var (
	sharedIC *Intracom
)

func TestMain(m *testing.M) {
	// setup
	sharedIC = New("test-intracom")
	m.Run()

	err := Close(sharedIC)
	if err != nil {
		panic(err)
	}
}

func TestIntracom_CreateTopicWhileClosed(t *testing.T) {
	ic := New("test-intracom")
	err := Close(ic)
	if err != nil {
		t.Fatalf("error closing intracom: %v", err)
	}

	_, err = CreateTopic[string](ic, TopicConfig{
		Name: t.Name(),
		// Buffer:      1,
		ErrIfExists: true,
	})

	if err == nil {
		t.Fatalf("error creating topic while closed should not be nil")
	}

}

func TestIntracom_RemoveTopicWhileClosed(t *testing.T) {
	// create an intracom registry
	ic := New("test-intracom")

	_, err := CreateTopic[string](ic, TopicConfig{
		Name: t.Name(),
		// Buffer:      1,
		ErrIfExists: true,
	})

	if err != nil {
		t.Fatalf("error creating topic: %v", err)
	}

	// close early
	err = Close(ic)
	if err != nil {
		t.Fatalf("error closing intracom: %v", err)
	}

	err = RemoveTopic[string](ic, t.Name())
	if err == nil {
		t.Fatalf("error removing topic while closed should not be nil")
	}

	if Close(ic) == nil {
		t.Fatalf("error closing topic while closed should not be nil")
	}

}

func TestIntracom_CreateTopicUnique(t *testing.T) {

	testTopic, err := CreateTopic[string](sharedIC, TopicConfig{
		Name: t.Name(),
		// Buffer:      1,
		ErrIfExists: true,
	})

	if err != nil {
		t.Fatalf("error creating topic: %v", err)
	}

	if testTopic.PublishChannel() == nil {
		t.Fatalf("topic publisher should not be nil")
	}
}

func TestIntracom_CreateTopicDuplicate(t *testing.T) {

	_, err := CreateTopic[string](sharedIC, TopicConfig{
		Name: t.Name(),
		// Buffer:      1,
		ErrIfExists: true,
	})
	if err != nil {
		t.Fatalf("error creating topic: %v", err)
	}

	_, err = CreateTopic[string](sharedIC, TopicConfig{
		Name: t.Name(),
		// Buffer:      1,
		ErrIfExists: true,
	})

	if err == nil {
		t.Fatalf("error creating duplicate topic should not be nil")
	}

}

func TestIntracom_RemoveTopic(t *testing.T) {

	testTopic, err := CreateTopic[string](sharedIC, TopicConfig{
		Name: t.Name(),
		// Buffer:      1,
		ErrIfExists: true,
	})
	if err != nil {
		t.Fatalf("error creating topic: %v", err)
	}

	err = RemoveTopic[string](sharedIC, t.Name())
	if err != nil {
		t.Fatalf("error removing topic: %v", err)
	}

	if testTopic.Close() == nil {
		t.Fatalf("error closing topic should not be nil")
	}
}

func TestIntracom_RemoveNonExistentTopic(t *testing.T) {

	testTopic, err := CreateTopic[string](sharedIC, TopicConfig{
		Name: t.Name(),
		// Buffer:      1,
		ErrIfExists: true,
	})
	if err != nil {
		t.Fatalf("error creating topic: %v", err)
	}

	err = RemoveTopic[string](sharedIC, "doesnt-exist")
	if err == nil {
		t.Fatalf("error removing non-existent topic should not be nil")
	}

	if testTopic.Close() != nil {
		t.Fatalf("error closing topic should be nil")
	}

}

func TestIntracom_CreateSubscriptionWithTopic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	testTopic, err := CreateTopic[string](sharedIC, TopicConfig{
		Name: t.Name(),
		// Buffer:      1,
		ErrIfExists: true,
	})

	if err != nil {
		t.Fatalf("error creating topic: %v", err)
	}

	if testTopic.PublishChannel() == nil {
		t.Fatalf("topic publisher should not be nil")
	}

	sub, err := CreateSubscription[string](ctx, sharedIC, t.Name(), 0, SubscriberConfig{
		ConsumerGroup: t.Name(),
		BufferSize:    1,
		BufferPolicy:  DropNone,
	})

	if err != nil {
		t.Fatalf("error creating subscription: %v", err)
	}

	if sub == nil {
		t.Fatalf("subscription channel should not be nil")
	}

}

func TestIntracom_CreateSubscriptionWithNoTopic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	sub, err := CreateSubscription[string](ctx, sharedIC, t.Name(), 0, SubscriberConfig{
		ConsumerGroup: t.Name(),
		BufferSize:    1,
		BufferPolicy:  DropNone,
	})

	if err == nil {
		t.Fatalf("error creating subscription should not be nil")
	}

	if sub != nil {
		t.Fatalf("subscription channel should be nil")
	}
}

func TestIntracom_RemoveSubscriptionFromTopic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	topic, err := CreateTopic[string](sharedIC, TopicConfig{
		Name: t.Name(),
		// Buffer:      1,
		ErrIfExists: true,
	})

	if err != nil {
		t.Fatalf("error creating topic: %v", err)
	}

	if topic.PublishChannel() == nil {
		t.Fatalf("topic publisher should not be nil")
	}

	sub, err := CreateSubscription[string](ctx, sharedIC, t.Name(), 0, SubscriberConfig{
		ConsumerGroup: t.Name(),
		BufferSize:    1,
		BufferPolicy:  DropNone,
	})

	if err != nil {
		t.Fatalf("error creating subscription: %v", err)
	}

	if sub == nil {
		t.Fatalf("subscription channel should not be nil")
	}

	err = RemoveSubscription(sharedIC, t.Name(), t.Name(), sub)
	if err != nil {
		t.Fatalf("error removing subscription: %v", err)
	}

}

func TestIntracom_Close(t *testing.T) {
	ic := New("test-intracom")

	err := Close(ic)
	if err != nil {
		t.Fatalf("error closing intracom: %v", err)
	}

	err = Close(ic)
	if err == nil {
		t.Fatalf("error closing intracom should not be nil")
	}

}

func TestIntracom_SinglePublisherSingleSubscriber(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ic := New("test-intracom")
	topicName := t.Name()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		// attempt to subscribe to a topic that may not exist yet waiting up to maxWait
		defer wg.Done()
		maxWait := 500 * time.Millisecond
		// create a subscription, waiting for topic to exist using maxWait
		sub, err := CreateSubscription[int](ctx, ic, topicName, maxWait, SubscriberConfig{
			ConsumerGroup: t.Name(), // unique consumer group name
			ErrIfExists:   true,     // error if consumer group already exists
			BufferSize:    1,        // subscriber channel buffer size
			BufferPolicy:  DropNone, // policy for handling buffer overflow
		})

		if err != nil {
			t.Errorf("error creating the subscription: %s", err)
			return
		}
		defer RemoveSubscription(ic, topicName, t.Name(), sub)

		if sub == nil {
			t.Errorf("subscription channel should not be nil")
		}

		var total int

		open := true
		for open {
			select {
			case _, open = <-sub:
				if !open {
					// prevent incrementing counter on closed channel
					return
				}

				total++
			case <-ctx.Done():
				t.Errorf("timeout waiting for message")
				open = false
			}
		}

		if total != 10 {
			t.Errorf("expected 10 messages, got %d", total)
		}
	}()

	go func() {
		// launch a publisher
		defer wg.Done()
		// slight delay so create subscription has to wait
		testTopic, err := CreateTopic[int](ic, TopicConfig{
			Name: topicName,
			// Buffer:               1,
			ErrIfExists:          true,
			SubscriberAwareCount: 1, // dont publish until you see a subscriber.
		})

		if err != nil {
			t.Errorf("error creating topic: %v", err)
			return
		}

		pubC := testTopic.PublishChannel()

		publishing := true
		for count := 0; publishing && count < 10; count++ {
			select {
			case <-ctx.Done():
				t.Errorf("publisher timed out waiting for subscriber")
				publishing = false
			case pubC <- count:
			}
		}

		err = testTopic.Close()
		if err != nil {
			t.Errorf("error closing topic: %v", err)
		}
	}()

	wg.Wait()

	err := Close(ic)
	if err != nil {
		t.Fatalf("error closing intracom: %v", err)
	}
}

func BenchmarkIntracom_2Subscriber1Publisher(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ic := New("benchmark-intracom")
	topicName := b.Name()
	maxSubscriberWait := 500 * time.Millisecond

	var wg sync.WaitGroup
	wg.Add(3) // 1 publisher, 2 subscribers

	go func() {
		// attempt to subscribe to a topic that may not exist yet waiting up to maxWait
		defer wg.Done()
		subscriberName := b.Name() + "-1"

		// create a subscription, waiting for topic to exist using maxWait
		sub, err := CreateSubscription[int](ctx, ic, topicName, maxSubscriberWait, SubscriberConfig{
			ConsumerGroup: subscriberName, // unique consumer group name
			ErrIfExists:   true,           // error if consumer group already exists
			BufferSize:    1,              // subscriber channel buffer size
			BufferPolicy:  DropNone,       // policy for handling buffer overflow
		})

		if err != nil {
			b.Errorf("error creating the subscription: %s", err)
			return
		}
		defer RemoveSubscription(ic, topicName, subscriberName, sub)

		if sub == nil {
			b.Errorf("subscription channel should not be nil")
		}

		var total int

		open := true
		for open {
			select {
			case _, open = <-sub:
				if !open {
					// prevent incrementing counter on closed channel
					return
				}

				total++
			case <-ctx.Done():
				b.Errorf("timeout waiting for message")
				open = false
			}
		}

		if total != b.N {
			b.Errorf("expected %d messages, got %d", b.N, total)
		}
	}()

	go func() {
		// attempt to subscribe to a topic that may not exist yet waiting up to maxWait
		defer wg.Done()

		subscriberName := b.Name() + "-2"

		// create a subscription, waiting for topic to exist using maxWait
		sub, err := CreateSubscription[int](ctx, ic, topicName, maxSubscriberWait, SubscriberConfig{
			ConsumerGroup: subscriberName, // unique consumer group name
			ErrIfExists:   true,           // error if consumer group already exists
			BufferSize:    1,              // subscriber channel buffer size
			BufferPolicy:  DropNone,       // policy for handling buffer overflow
		})

		if err != nil {
			b.Errorf("error creating the subscription: %s", err)
			return
		}
		defer RemoveSubscription(ic, topicName, subscriberName, sub)

		if sub == nil {
			b.Errorf("subscription channel should not be nil")
		}

		var total int

		open := true
		for open {
			select {
			case _, open = <-sub:
				if !open {
					// prevent incrementing counter on closed channel
					return
				}

				total++
			case <-ctx.Done():
				b.Errorf("timeout waiting for message")
				open = false
			}
		}

		if total != b.N {
			b.Errorf("expected %d messages, got %d", b.N, total)
		}
	}()

	go func() {
		// launch a publisher
		defer wg.Done()
		// slight delay so create subscription has to wait
		testTopic, err := CreateTopic[int](ic, TopicConfig{
			Name: topicName,
			// Buffer:               1000,
			ErrIfExists:          true,
			SubscriberAwareCount: 2, // dont publish until you see 2 subscribers.
		})

		if err != nil {
			b.Errorf("error creating topic: %v", err)
			return
		}

		pubC := testTopic.PublishChannel()

		publishing := true
		for count := 0; publishing && count < 1_000_000; count++ {
			select {
			case <-ctx.Done():
				b.Errorf("publisher timed out waiting for subscriber")
				publishing = false
			case pubC <- count:
			}
		}

		err = testTopic.Close()
		if err != nil {
			b.Errorf("error closing topic: %v", err)
		}
	}()

	wg.Wait()

	err := Close(ic)
	if err != nil {
		b.Fatalf("error closing intracom: %v", err)
	}

}
