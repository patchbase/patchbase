package events

import (
	"sync"
	"sync/atomic"
)

// Event represents a state-change notification published by a write path.
type Event struct {
	Topic string // e.g. "hosts", "host:{id}", "advisories"
	Type  string // "updated", "snapshot", "matched", "deleted"
	Data  any    // optional payload for full-data pushes; nil for notification-only
}

// Subscriber identifies a subscription and provides the receive channel.
type Subscriber struct {
	ID     uint64
	Topics []string
	Events chan Event
}

// Broker is the pub/sub abstraction for distributing state-change events.
type Broker interface {
	// Subscribe registers interest in one or more topics and returns
	// a Subscriber with a receive-only channel of events.
	Subscribe(topics []string) *Subscriber
	// Unsubscribe removes a subscriber and closes its channel.
	Unsubscribe(sub *Subscriber)
	// Update changes the active topics for a subscriber without disconnecting it.
	Update(sub *Subscriber, topics []string)
	// Publish broadcasts an event to all subscribers of its topic.
	// Non-blocking: if a subscriber's buffer is full the event is dropped.
	Publish(e Event)
}

const subscriberBufferSize = 16

// localBroker is the in-memory implementation.
type localBroker struct {
	mu          sync.RWMutex
	subscribers map[string]map[uint64]chan Event // topic -> subscriberID -> chan
	channels    map[uint64]chan Event            // subscriberID -> chan (for cleanup)
	nextID      atomic.Uint64
}

// NewBroker returns an in-memory Broker.
func NewBroker() Broker {
	return &localBroker{
		subscribers: make(map[string]map[uint64]chan Event),
		channels:    make(map[uint64]chan Event),
		mu:          sync.RWMutex{},
		nextID:      atomic.Uint64{},
	}
}

func (b *localBroker) Subscribe(topics []string) *Subscriber {
	id := b.nextID.Add(1)
	ch := make(chan Event, subscriberBufferSize)

	b.mu.Lock()
	b.channels[id] = ch
	for _, topic := range topics {
		if b.subscribers[topic] == nil {
			b.subscribers[topic] = make(map[uint64]chan Event)
		}
		b.subscribers[topic][id] = ch
	}
	b.mu.Unlock()

	return &Subscriber{
		ID:     id,
		Topics: topics,
		Events: ch,
	}
}

func (b *localBroker) Unsubscribe(sub *Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Close the channel and remove from the channels map.
	ch, ok := b.channels[sub.ID]
	if ok {
		close(ch)
		delete(b.channels, sub.ID)
	}

	// Remove from all topic maps.
	for _, topic := range sub.Topics {
		if subs, ok := b.subscribers[topic]; ok {
			delete(subs, sub.ID)
			if len(subs) == 0 {
				delete(b.subscribers, topic)
			}
		}
	}
}

func (b *localBroker) Update(sub *Subscriber, topics []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Remove from old topics
	for _, topic := range sub.Topics {
		if subs, ok := b.subscribers[topic]; ok {
			delete(subs, sub.ID)
			if len(subs) == 0 {
				delete(b.subscribers, topic)
			}
		}
	}

	// Add to new topics
	for _, topic := range topics {
		if b.subscribers[topic] == nil {
			b.subscribers[topic] = make(map[uint64]chan Event)
		}
		b.subscribers[topic][sub.ID] = sub.Events
	}

	sub.Topics = topics
}

func (b *localBroker) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs := b.subscribers[e.Topic]
	if subs == nil {
		return
	}

	for _, ch := range subs {
		select {
		case ch <- e:
		default:
			// Subscriber buffer full — drop the event. The subscriber
			// will re-fetch on the next event it does receive.
		}
	}
}
