package domain

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// WebhookEvent represents an event that can trigger webhook deliveries.
type WebhookEvent struct {
	Type    string    // e.g., "user.created", "invoice.paid"
	Payload any       // Event payload data
	OrgID   *uuid.UUID // Optional: organization context for org-scoped webhook matching
}

// EventHandler is a function that processes webhook events.
type EventHandler func(event WebhookEvent)

// EventBus provides in-process event publishing and subscription using Go channels.
// It decouples event producers from webhook delivery logic.
type EventBus struct {
	mu       sync.RWMutex
	buffer   int
	handlers []EventHandler
	ch       chan WebhookEvent
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewEventBus creates a new EventBus with the specified buffer size.
// If bufferSize is <= 0, a default of 256 is used.
func NewEventBus(bufferSize int) *EventBus {
	if bufferSize <= 0 {
		bufferSize = 256
	}
	return &EventBus{
		buffer:   bufferSize,
		handlers: make([]EventHandler, 0),
		ch:       make(chan WebhookEvent, bufferSize),
	}
}

// Start begins the dispatch goroutine that reads events from the channel
// and calls all registered handlers sequentially.
func (eb *EventBus) Start(ctx context.Context) error {
	eb.ctx, eb.cancel = context.WithCancel(ctx)
	eb.wg.Add(1)
	go eb.dispatch()
	return nil
}

// Stop cancels the context and waits for the dispatch goroutine to drain
// with a 5-second timeout.
func (eb *EventBus) Stop() error {
	if eb.cancel != nil {
		eb.cancel()
	}
	done := make(chan struct{})
	go func() {
		eb.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		return context.DeadlineExceeded
	}
}

// Subscribe registers a new event handler. It is safe for concurrent use.
func (eb *EventBus) Subscribe(handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.handlers = append(eb.handlers, handler)
}

// Publish sends an event to the channel. It is non-blocking and returns an error
// if the channel is full or the context has been cancelled.
func (eb *EventBus) Publish(event WebhookEvent) error {
	if eb.ctx == nil {
		return errors.New("event bus not started")
	}
	select {
	case <-eb.ctx.Done():
		return eb.ctx.Err()
	default:
	}
	select {
	case eb.ch <- event:
		return nil
	default:
		return errors.New("event bus channel full")
	}
}

// dispatch reads events from the channel and calls all registered handlers.
// It recovers from panics in handlers to prevent crashing the bus.
func (eb *EventBus) dispatch() {
	defer eb.wg.Done()
	for {
		select {
		case <-eb.ctx.Done():
			// Drain remaining events before shutting down.
			for {
				select {
				case event := <-eb.ch:
					eb.callHandlers(event)
				default:
					return
				}
			}
		case event := <-eb.ch:
			eb.callHandlers(event)
		}
	}
}

// callHandlers invokes all registered handlers sequentially for the given event.
// It recovers from panics in individual handlers.
func (eb *EventBus) callHandlers(event WebhookEvent) {
	eb.mu.RLock()
	handlers := make([]EventHandler, len(eb.handlers))
	copy(handlers, eb.handlers)
	eb.mu.RUnlock()
	for _, h := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("eventbus: handler panic recovered: %v", r)
				}
			}()
			h(event)
		}()
	}
}
