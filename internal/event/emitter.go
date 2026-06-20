package event

import (
	"sync"

	"github.com/sirupsen/logrus"
)

// Subscriber defines the interface for event subscribers.
type Subscriber interface {
	// Handle processes an event.
	Handle(evt Event)
}

// Emitter publishes events to synchronous and asynchronous subscribers.
type Emitter struct {
	syncSubs  []Subscriber
	asyncSubs []Subscriber
	asyncChan chan Event
	wg        sync.WaitGroup
	mu        sync.RWMutex
	closed    bool
	logger    *logrus.Entry
}

// SetLogger attaches a structured logger to the emitter.
// When set, dropped events (channel full) are logged at Warn level instead of
// silently discarded, making back-pressure visible in production logs.
func (e *Emitter) SetLogger(logger *logrus.Entry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logger = logger
}

// NewEmitter creates a new event emitter.
func NewEmitter() *Emitter {
	e := &Emitter{
		asyncChan: make(chan Event, 256),
	}
	// Start background goroutine to process async events
	e.wg.Add(1)
	go e.asyncWorker()
	return e
}

// Subscribe registers a synchronous subscriber.
func (e *Emitter) Subscribe(subscriber Subscriber) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.syncSubs = append(e.syncSubs, subscriber)
}

// SubscribeAsync registers an asynchronous subscriber.
func (e *Emitter) SubscribeAsync(subscriber Subscriber) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.asyncSubs = append(e.asyncSubs, subscriber)
}

// safeHandle dispatches an event to one subscriber with panic isolation, so a
// faulty subscriber can never crash the publishing goroutine or the async worker
// (OBS-1). Panics are logged and swallowed.
func (e *Emitter) safeHandle(sub Subscriber, evt Event) {
	defer func() {
		if r := recover(); r != nil && e.logger != nil {
			e.logger.WithFields(logrus.Fields{
				"event_type": evt.Type,
				"panic":      r,
			}).Error("event subscriber panicked; recovered")
		}
	}()
	sub.Handle(evt)
}

// Publish sends an event to all registered subscribers.
func (e *Emitter) Publish(evt Event) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return
	}

	// Call synchronous subscribers directly (panic-isolated).
	for _, sub := range e.syncSubs {
		e.safeHandle(sub, evt)
	}

	// Send to async channel for asynchronous processing
	select {
	case e.asyncChan <- evt:
	default:
		// Channel full — drop event and log so back-pressure is visible in production.
		if e.logger != nil {
			e.logger.WithFields(logrus.Fields{
				"event_type": evt.Type,
				"tenant_id":  evt.TenantID,
			}).Warn("event emitter channel full: audit event dropped")
		}
	}
}

// asyncWorker processes events from the async channel.
func (e *Emitter) asyncWorker() {
	defer e.wg.Done()
	for evt := range e.asyncChan {
		e.mu.RLock()
		asyncSubs := make([]Subscriber, len(e.asyncSubs))
		copy(asyncSubs, e.asyncSubs)
		e.mu.RUnlock()

		for _, sub := range asyncSubs {
			e.safeHandle(sub, evt)
		}
	}
}

// Close gracefully shuts down the emitter.
// It closes the async channel and waits for pending events to be processed.
func (e *Emitter) Close() {
	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()

	close(e.asyncChan)
	e.wg.Wait()
}
