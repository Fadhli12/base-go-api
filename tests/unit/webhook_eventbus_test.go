package unit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventBus_NewEventBus tests the EventBus constructor with various buffer sizes.
func TestEventBus_NewEventBus(t *testing.T) {
	t.Run("default buffer size when zero", func(t *testing.T) {
		bus := domain.NewEventBus(0)
		assert.NotNil(t, bus)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := bus.Start(ctx)
		require.NoError(t, err)
		defer bus.Stop()
	})

	t.Run("default buffer size when negative", func(t *testing.T) {
		bus := domain.NewEventBus(-1)
		assert.NotNil(t, bus)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := bus.Start(ctx)
		require.NoError(t, err)
		defer bus.Stop()
	})

	t.Run("custom buffer size", func(t *testing.T) {
		bus := domain.NewEventBus(64)
		assert.NotNil(t, bus)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := bus.Start(ctx)
		require.NoError(t, err)
		defer bus.Stop()
	})
}

// TestEventBus_PublishSubscribe tests the basic publish/subscribe flow.
func TestEventBus_PublishSubscribe(t *testing.T) {
	t.Run("handler receives published event", func(t *testing.T) {
		bus := domain.NewEventBus(10)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := bus.Start(ctx)
		require.NoError(t, err)

		var received domain.WebhookEvent
		var wg sync.WaitGroup
		wg.Add(1)

		bus.Subscribe(func(event domain.WebhookEvent) {
			received = event
			wg.Done()
		})

		event := domain.WebhookEvent{
			Type:    "user.created",
			Payload: map[string]string{"email": "test@example.com"},
		}

		err = bus.Publish(event)
		require.NoError(t, err)

		// Wait for async handler
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for handler")
		}

		assert.Equal(t, "user.created", received.Type)
		assert.NotNil(t, received.Payload)

		err = bus.Stop()
		require.NoError(t, err)
	})

	t.Run("multiple handlers receive same event", func(t *testing.T) {
		bus := domain.NewEventBus(10)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := bus.Start(ctx)
		require.NoError(t, err)

		var counter atomic.Int32
		var wg sync.WaitGroup

		handlerCount := 3
		wg.Add(handlerCount)

		handler := func(event domain.WebhookEvent) {
			counter.Add(1)
			wg.Done()
		}

		for i := 0; i < handlerCount; i++ {
			bus.Subscribe(handler)
		}

		err = bus.Publish(domain.WebhookEvent{Type: "user.created"})
		require.NoError(t, err)

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for all handlers")
		}

		assert.Equal(t, int32(handlerCount), counter.Load())

		err = bus.Stop()
		require.NoError(t, err)
	})

	t.Run("event with OrgID", func(t *testing.T) {
		bus := domain.NewEventBus(10)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := bus.Start(ctx)
		require.NoError(t, err)

		orgID := uuid.New()
		var receivedOrgID *uuid.UUID
		var wg sync.WaitGroup
		wg.Add(1)

		bus.Subscribe(func(event domain.WebhookEvent) {
			receivedOrgID = event.OrgID
			wg.Done()
		})

		err = bus.Publish(domain.WebhookEvent{
			Type:    "invoice.paid",
			Payload: map[string]int{"amount": 100},
			OrgID:   &orgID,
		})
		require.NoError(t, err)

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for handler")
		}

		require.NotNil(t, receivedOrgID)
		assert.Equal(t, orgID, *receivedOrgID)

		err = bus.Stop()
		require.NoError(t, err)
	})
}

// TestEventBus_PublishBeforeStart tests that publishing before Start returns error.
func TestEventBus_PublishBeforeStart(t *testing.T) {
	bus := domain.NewEventBus(10)
	err := bus.Publish(domain.WebhookEvent{Type: "user.created"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

// TestEventBus_ConcurrentPublish tests concurrent publish into the bus.
func TestEventBus_ConcurrentPublish(t *testing.T) {
	bus := domain.NewEventBus(1024)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := bus.Start(ctx)
	require.NoError(t, err)

	var handled atomic.Int32
	bus.Subscribe(func(event domain.WebhookEvent) {
		handled.Add(1)
	})

	numEvents := 100
	var wg sync.WaitGroup
	wg.Add(numEvents)

	for i := 0; i < numEvents; i++ {
		go func(idx int) {
			defer wg.Done()
			err := bus.Publish(domain.WebhookEvent{
				Type:    "user.created",
				Payload: idx,
			})
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Give the dispatch goroutine time to process all events
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int32(numEvents), handled.Load())

	err = bus.Stop()
	require.NoError(t, err)
}

// TestEventBus_ChannelFull tests behavior when the event channel is full.
func TestEventBus_ChannelFull(t *testing.T) {
	bus := domain.NewEventBus(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := bus.Start(ctx)
	require.NoError(t, err)

	blockCh := make(chan struct{})
	received := make(chan struct{}, 1)

	bus.Subscribe(func(event domain.WebhookEvent) {
		received <- struct{}{}
		<-blockCh
	})

	err = bus.Publish(domain.WebhookEvent{Type: "event.one"})
	require.NoError(t, err)

	<-received

	err = bus.Publish(domain.WebhookEvent{Type: "event.two"})
	require.NoError(t, err, "event.two should fill the 1-slot buffer while handler is busy on event.one")

	time.Sleep(100 * time.Millisecond)

	err = bus.Publish(domain.WebhookEvent{Type: "event.three"})
	assert.Error(t, err, "overflow publish should fail")
	assert.Contains(t, err.Error(), "channel full")

	close(blockCh)
	bus.Stop()
}

// TestEventBus_HandlerPanicRecovery tests that handler panics are recovered.
func TestEventBus_HandlerPanicRecovery(t *testing.T) {
	bus := domain.NewEventBus(10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := bus.Start(ctx)
	require.NoError(t, err)

	var secondHandlerCalled atomic.Bool

	// Panicky handler
	bus.Subscribe(func(event domain.WebhookEvent) {
		panic("intentional handler panic")
	})

	// This handler should still execute
	bus.Subscribe(func(event domain.WebhookEvent) {
		secondHandlerCalled.Store(true)
	})

	err = bus.Publish(domain.WebhookEvent{Type: "user.created"})
	require.NoError(t, err)

	// Give dispatch time
	time.Sleep(100 * time.Millisecond)

	assert.True(t, secondHandlerCalled.Load(), "second handler should be called despite first handler panic")

	err = bus.Stop()
	require.NoError(t, err)
}

// TestEventBus_StopDraining tests that Stop drains remaining events before returning.
func TestEventBus_StopDraining(t *testing.T) {
	bus := domain.NewEventBus(50)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := bus.Start(ctx)
	require.NoError(t, err)

	var handled atomic.Int32

	// Slow handler to test draining
	bus.Subscribe(func(event domain.WebhookEvent) {
		time.Sleep(10 * time.Millisecond)
		handled.Add(1)
	})

	numEvents := 20
	for i := 0; i < numEvents; i++ {
		err := bus.Publish(domain.WebhookEvent{
			Type:    "user.created",
			Payload: i,
		})
		require.NoError(t, err)
	}

	// Stop should drain all remaining events
	err = bus.Stop()
	require.NoError(t, err)

	assert.Equal(t, int32(numEvents), handled.Load(), "all events should be handled before stop returns")
}

// TestEventBus_StopDrainingTimeout tests that Stop times out if handlers are too slow.
func TestEventBus_StopDrainingTimeout(t *testing.T) {
	bus := domain.NewEventBus(10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := bus.Start(ctx)
	require.NoError(t, err)

	// Blocking handler
	blockCh := make(chan struct{})
	bus.Subscribe(func(event domain.WebhookEvent) {
		<-blockCh // Blocks forever
	})

	err = bus.Publish(domain.WebhookEvent{Type: "user.created"})
	require.NoError(t, err)

	// Stop with timeout
	stopErr := bus.Stop()

	// Unblock the handler
	close(blockCh)

	assert.Error(t, stopErr, "stop should timeout when handler blocks")
}

// TestEventBus_SubscribeConcurrent tests concurrent subscribe calls.
func TestEventBus_SubscribeConcurrent(t *testing.T) {
	bus := domain.NewEventBus(32)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := bus.Start(ctx)
	require.NoError(t, err)

	var wg sync.WaitGroup
	numHandlers := 50
	wg.Add(numHandlers)

	for i := 0; i < numHandlers; i++ {
		go func() {
			defer wg.Done()
			bus.Subscribe(func(event domain.WebhookEvent) {})
		}()
	}

	wg.Wait()

	// Publish an event - all handlers should survive
	err = bus.Publish(domain.WebhookEvent{Type: "user.created"})
	require.NoError(t, err)

	err = bus.Stop()
	require.NoError(t, err)
}

// TestEventBus_PublishAfterContextCancelled tests publishing after parent context is cancelled.
func TestEventBus_PublishAfterContextCancelled(t *testing.T) {
	bus := domain.NewEventBus(10)
	ctx, cancel := context.WithCancel(context.Background())
	err := bus.Start(ctx)
	require.NoError(t, err)

	// Cancel the context
	cancel()

	err = bus.Publish(domain.WebhookEvent{Type: "user.created"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "canceled")

	err = bus.Stop()
	assert.NoError(t, err)
}

// TestEventBus_StopWithoutStart tests calling Stop on a bus that was never started.
func TestEventBus_StopWithoutStart(t *testing.T) {
	bus := domain.NewEventBus(10)
	err := bus.Stop()
	assert.NoError(t, err)
}

// TestEventBus_MultipleStops tests calling Stop multiple times.
func TestEventBus_MultipleStops(t *testing.T) {
	bus := domain.NewEventBus(10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := bus.Start(ctx)
	require.NoError(t, err)

	err = bus.Stop()
	require.NoError(t, err)

	// Second stop should be safe
	err = bus.Stop()
	assert.NoError(t, err)
}

// TestEventBus_EventsOrdered tests that events are dispatched in order.
func TestEventBus_EventsOrdered(t *testing.T) {
	bus := domain.NewEventBus(50)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := bus.Start(ctx)
	require.NoError(t, err)

	var events []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	numEvents := 20
	wg.Add(numEvents)

	bus.Subscribe(func(event domain.WebhookEvent) {
		mu.Lock()
		events = append(events, event.Type)
		mu.Unlock()
		wg.Done()
	})

	for i := 0; i < numEvents; i++ {
		err := bus.Publish(domain.WebhookEvent{Type: "user.created", Payload: i})
		require.NoError(t, err)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for handlers")
	}

	mu.Lock()
	defer mu.Unlock()

	assert.Len(t, events, numEvents)
	for _, e := range events {
		assert.Equal(t, "user.created", e)
	}
}
