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
			Name:                 topicName,
			Buffer:               1,
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

func TestIntracom_CreateTopicWhileClosed(t *testing.T) {
	ic := New("test-intracom")
	err := Close(ic)
	if err != nil {
		t.Fatalf("error closing intracom: %v", err)
	}

	_, err = CreateTopic[string](ic, TopicConfig{
		Name:        t.Name(),
		Buffer:      1,
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
		Name:        t.Name(),
		Buffer:      1,
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
		Name:        t.Name(),
		Buffer:      1,
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
		Name:        t.Name(),
		Buffer:      1,
		ErrIfExists: true,
	})
	if err != nil {
		t.Fatalf("error creating topic: %v", err)
	}

	_, err = CreateTopic[string](sharedIC, TopicConfig{
		Name:        t.Name(),
		Buffer:      1,
		ErrIfExists: true,
	})

	if err == nil {
		t.Fatalf("error creating duplicate topic should not be nil")
	}

}

func TestIntracom_RemoveTopic(t *testing.T) {

	testTopic, err := CreateTopic[string](sharedIC, TopicConfig{
		Name:        t.Name(),
		Buffer:      1,
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
		Name:        t.Name(),
		Buffer:      1,
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
		Name:        t.Name(),
		Buffer:      1,
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
		Name:        t.Name(),
		Buffer:      1,
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

// func TestMultipleUnSubscribes(t *testing.T) {
// 	ic := New[bool]("test-intracom")

// 	err := ic.Start()
// 	if err != nil {
// 		t.Errorf("intracom start error should be nilt: want nil, got %v", err)
// 	}

// 	defer ic.Close()

// 	topic := "test-topic"
// 	group := "test-subscriber"

// 	conf := &SubscriberConfig{
// 		Topic:         topic,
// 		ConsumerGroup: group,
// 		BufferSize:    1,
// 		BufferPolicy:  DropNone,
// 	}

// 	_, unsubscribe := ic.Subscribe(conf)
// 	unsubscribe()
// 	unsubscribe()
// }

// func TestLateSubscriberDuringSignalCancel(t *testing.T) {
// 	ic := New[bool]("test-intracom")

// 	err := ic.Start()
// 	if err != nil {
// 		t.Errorf("intracom start error should be nilt: want nil, got %v", err)
// 	}

// 	defer ic.Close()

// 	topic := "test-topic"
// 	group1 := "test-subscriber1"
// 	group2 := "test-subscriber2"

// 	conf1 := &SubscriberConfig{
// 		Topic:         topic,
// 		ConsumerGroup: group1,
// 		BufferSize:    1,
// 		BufferPolicy:  DropNone,
// 	}

// 	conf2 := &SubscriberConfig{
// 		Topic:         topic,
// 		ConsumerGroup: group2,
// 		BufferSize:    1,
// 		BufferPolicy:  DropNone,
// 	}

// 	var wg sync.WaitGroup
// 	wg.Add(3)

// 	doneC := make(chan struct{})

// 	publishC1, unregister := ic.Register(topic)

// 	go func() {
// 		defer wg.Done()
// 		timer := time.NewTimer(2 * time.Second)
// 		defer timer.Stop()
// 		for {
// 			select {
// 			case <-timer.C:
// 				unregister()
// 				close(doneC)
// 				return
// 			case publishC1 <- true:
// 			}
// 		}
// 	}()

// 	go func() {
// 		defer wg.Done()

// 		ch1, _ := ic.Subscribe(conf1)
// 		// defer unsubscribe1()
// 		var isDone bool
// 		for !isDone {
// 			select {
// 			case <-doneC:
// 				isDone = true
// 			case <-ch1:
// 				// w/e
// 			}
// 		}
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		time.Sleep(3 * time.Second)
// 		ch2, _ := ic.Subscribe(conf2)

// 		var isDone bool
// 		for !isDone {
// 			select {
// 			case <-doneC:
// 				isDone = true
// 			case <-ch2:
// 			}
// 		}
// 	}()

// 	wg.Wait()

// 	want := false
// 	// consumer one should have been removed by unregister process
// 	_, got1 := ic.get(topic, group1)
// 	if want != got1 {
// 		t.Errorf("subscriber exists: want %v, got %v", want, got1)
// 	}

// 	want = true
// 	// late subscriber after unregister should be added again
// 	_, got2 := ic.get(topic, group2)
// 	if want != got2 {
// 		t.Errorf("subscriber exists: want %v, got %v", want, got2)
// 	}

// }

// func TestIntracomCloseWithoutUnsubscribing(t *testing.T) {
// 	ic := New[bool]("test-intracom")

// 	err := ic.Start()
// 	if err != nil {
// 		t.Errorf("intracom start error should be nilt: want nil, got %v", err)
// 	}

// 	defer ic.Close()

// 	topic := "test-topic"
// 	group := "test-subscriber"

// 	conf := &SubscriberConfig{
// 		Topic:         topic,
// 		ConsumerGroup: group,
// 		BufferSize:    1,
// 		BufferPolicy:  DropNone,
// 	}

// 	// subscribe and we should receive an immediate message if there is a message in the last message map
// 	ic.Subscribe(conf)

// 	want := true
// 	_, got := ic.get(topic, group) // true if exists
// 	if want != got {
// 		t.Errorf("want %v, got %v", want, got)
// 	}

// 	// intracom instance unusable, sending requests will panic
// 	ic.Close()

// 	want = false
// 	_, got = <-ic.requestC
// 	if want != got {
// 		t.Errorf("intracom requests channel open: want %v, got %v", want, got)
// 	}

// }

// // Testing typed instance creations
// func TestNewBoolTyped(t *testing.T) {
// 	ic := New[bool]("test-intracom")

// 	want := reflect.TypeOf(new(Intracom[bool])).String()
// 	got := reflect.TypeOf(ic).String()

// 	if want != got {
// 		t.Errorf("want %s: got %s", want, got)
// 	}

// }

// func TestNewStringTyped(t *testing.T) {

// 	ic := New[string]("test-intracom")

// 	want := reflect.TypeOf(new(Intracom[string])).String()
// 	got := reflect.TypeOf(ic).String()

// 	if want != got {
// 		t.Errorf("want %s: got %s", want, got)
// 	}

// }

// func TestNewIntTyped(t *testing.T) {
// 	ic := New[int]("test-intracom")

// 	want := reflect.TypeOf(new(Intracom[int])).String()
// 	got := reflect.TypeOf(ic).String()

// 	if want != got {
// 		t.Errorf("want %s: got %s", want, got)
// 	}

// }

// func TestNewByteTyped(t *testing.T) {

// 	ic := New[[]byte]("test-intracom")

// 	want := reflect.TypeOf(new(Intracom[[]byte])).String()
// 	got := reflect.TypeOf(ic).String()

// 	if want != got {
// 		t.Errorf("want %s: got %s", want, got)
// 	}

// }

// func countMessages[T any](num int, sub <-chan T, subCh chan int) {
// 	var total int
// 	for range sub {
// 		total++
// 	}
// 	subCh <- total
// }

// func BenchmarkIntracom(b *testing.B) {
// 	ic := New[string]("test-intracom")

// 	err := ic.Start()
// 	if err != nil {
// 		b.Errorf("intracom start error should be nilt: want nil, got %v", err)
// 	}

// 	defer ic.Close()

// 	topic := "channel1"

// 	totalSub1 := make(chan int, 1)
// 	totalSub2 := make(chan int, 1)
// 	totalSub3 := make(chan int, 1)

// 	var wg sync.WaitGroup
// 	wg.Add(4)

// 	go func() {
// 		defer wg.Done()
// 		sub1, unsubscribe := ic.Subscribe(&SubscriberConfig{
// 			Topic:         topic,
// 			ConsumerGroup: "sub1",
// 			BufferSize:    10,
// 			BufferPolicy:  DropNone,
// 		})

// 		defer unsubscribe()

// 		countMessages[string](b.N, sub1, totalSub1)
// 		// fmt.Println("sub1 done")
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		sub2, unsubscribe := ic.Subscribe(&SubscriberConfig{
// 			Topic:         topic,
// 			ConsumerGroup: "sub2",
// 			BufferSize:    10,
// 			BufferPolicy:  DropNone,
// 		})
// 		defer unsubscribe()

// 		countMessages[string](b.N, sub2, totalSub2)
// 		// fmt.Println("sub2 done")
// 	}()

// 	go func() {
// 		defer wg.Done()

// 		sub3, unsubscribe := ic.Subscribe(&SubscriberConfig{
// 			Topic:         topic,
// 			ConsumerGroup: "sub3",
// 			BufferSize:    10,
// 			BufferPolicy:  DropNone,
// 		})
// 		defer unsubscribe()

// 		countMessages[string](b.N, sub3, totalSub3)
// 		// fmt.Println("sub3 done")
// 	}()

// 	// NOTE: this sleep is necessary to ensure that the subscribers receive all their messages.
// 	// without a publisher sleep, subscribers may not be subscribed early enough and would miss messages.
// 	time.Sleep(25 * time.Millisecond)
// 	b.ResetTimer() // reset benchmark timer once we launch the publisher

// 	go func() {
// 		defer wg.Done()

// 		publishCh, unregister := ic.Register(topic)
// 		defer unregister() // should be called only after done publishing otherwise it will panic

// 		for i := 0; i < b.N; i++ {
// 			publishCh <- "test message"
// 		}
// 	}()

// 	wg.Wait()

// 	ic.Close() // should be called last

// 	got1 := <-totalSub1
// 	if got1 != b.N {
// 		b.Errorf("expected %d total, got %d", b.N, got1)
// 	}

// 	got2 := <-totalSub2
// 	if got2 != b.N {
// 		b.Errorf("expected %d total, got %d", b.N, got2)
// 	}

// 	got3 := <-totalSub3
// 	if got3 != b.N {
// 		b.Errorf("expected %d total, got %d", b.N, got3)
// 	}

// 	b.StopTimer()

// }
