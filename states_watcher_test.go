package rxd

import (
	"context"
	"testing"
	"time"
)

func TestStateWatcher(t *testing.T) {
	// Create a new stateWatcher instance
	sw := newStateWatcher(stateWatcherConfig{
		bufferSize: 4,
		logger:     nil,
	})
	// watch must be running to pub/sub
	go sw.watch()

	defer sw.stop()

	t.Run("LateSubscriber", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		serviceName := "late-subscriber-service"
		// current publish
		sw.publish(ctx, serviceName, Init)

		// late subscribe
		consumer := "test-late-consumer"
		ch, err := sw.subscribe(ctx, consumer)
		if err != nil {
			t.Errorf("subscribe method returned an error: %v", err)
		}

	loop:
		for {
			select {
			case <-ctx.Done():
				t.Errorf("wait exceeded, context was cancelled: %v", err)
				return
			case states := <-ch:
				// keep watching the channel for the states
				if state, ok := states[serviceName]; ok {
					// check if a state is set for the service
					if state != Init {
						t.Errorf("expected state: %v, got: %v", Init, states[serviceName])
					}
					break loop
				}
			}
		}
		// Test unsubscribe method
		err = sw.unsubscribe(ctx, consumer)
		if err != nil {
			t.Errorf("unsubscribe method returned an error: %v", err)
		}

	})

	t.Run("EarlySubscriber", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		serviceName := "early-subscriber-service"

		// early subscribe
		consumer := "test-early-consumer"
		ch, err := sw.subscribe(ctx, consumer)
		if err != nil {
			t.Errorf("subscribe method returned an error: %v", err)
		}

		// current publish
		sw.publish(ctx, serviceName, Init)

	loop:
		for {
			select {
			case <-ctx.Done():
				t.Errorf("wait exceeded, context was cancelled: %v", err)
				return
			case states := <-ch:
				// keep watching the channel for the states
				if state, ok := states[serviceName]; ok {
					// check if a state is set for the service
					if state != Init {
						t.Errorf("expected state: %v, got: %v", Init, states[serviceName])
					}
					break loop
				}
			}
		}

		// Test unsubscribe method
		err = sw.unsubscribe(ctx, consumer)
		if err != nil {
			t.Errorf("unsubscribe method returned an error: %v", err)
		}

	})

}
