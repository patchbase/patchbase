// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscribeAndPublish(t *testing.T) {
	b := NewBroker()
	sub := b.Subscribe([]string{"hosts"})
	defer b.Unsubscribe(sub)

	b.Publish(NewHostsUpdatedEvent())

	select {
	case ev := <-sub.Events:
		assert.Equal(t, "hosts", ev.Topic)
		assert.Equal(t, "updated", ev.Type)
	default:
		require.Fail(t, "expected to receive event")
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	b := NewBroker()
	sub := b.Subscribe([]string{"hosts"})

	b.Unsubscribe(sub)

	_, ok := <-sub.Events
	assert.False(t, ok, "channel should be closed after unsubscribe")
}

func TestUnsubscribeRemovesFromAllTopics(t *testing.T) {
	b := NewBroker()
	sub := b.Subscribe([]string{"hosts", "advisories"})

	b.Unsubscribe(sub)

	b.Publish(NewHostsUpdatedEvent())
	b.Publish(NewAdvisoriesUpdatedEvent())

	_, ok := <-sub.Events
	assert.False(t, ok, "channel should be closed")
}

func TestPublishNoSubscribers(t *testing.T) {
	b := NewBroker()
	b.Publish(NewHostsUpdatedEvent())
}

func TestPublishMultipleSubscribers(t *testing.T) {
	b := NewBroker()
	sub1 := b.Subscribe([]string{"hosts"})
	sub2 := b.Subscribe([]string{"hosts"})
	defer b.Unsubscribe(sub1)
	defer b.Unsubscribe(sub2)

	b.Publish(NewHostsUpdatedEvent())

	for i, sub := range []*Subscriber{sub1, sub2} {
		select {
		case ev := <-sub.Events:
			assert.Equal(t, "hosts", ev.Topic, "subscriber %d", i)
		default:
			require.Fail(t, "subscriber %d did not receive event", i)
		}
	}
}

func TestTopicIsolation(t *testing.T) {
	b := NewBroker()
	subHosts := b.Subscribe([]string{"hosts"})
	subAdvisories := b.Subscribe([]string{"advisories"})
	defer b.Unsubscribe(subHosts)
	defer b.Unsubscribe(subAdvisories)

	b.Publish(NewHostsUpdatedEvent())

	select {
	case <-subAdvisories.Events:
		require.Fail(t, "advisories subscriber should not receive hosts event")
	default:
	}

	select {
	case ev := <-subHosts.Events:
		assert.Equal(t, "hosts", ev.Topic)
	default:
		require.Fail(t, "hosts subscriber should receive event")
	}
}

func TestUpdateAddTopics(t *testing.T) {
	b := NewBroker()
	sub := b.Subscribe([]string{"hosts"})
	defer b.Unsubscribe(sub)

	b.Update(sub, []string{"hosts", "advisories"})

	b.Publish(NewAdvisoriesUpdatedEvent())

	select {
	case ev := <-sub.Events:
		assert.Equal(t, "advisories", ev.Topic)
	default:
		require.Fail(t, "subscriber should receive event after topic update")
	}
}

func TestUpdateRemoveTopics(t *testing.T) {
	b := NewBroker()
	sub := b.Subscribe([]string{"hosts", "advisories"})
	defer b.Unsubscribe(sub)

	b.Update(sub, []string{"hosts"})

	b.Publish(NewAdvisoriesUpdatedEvent())

	select {
	case <-sub.Events:
		require.Fail(t, "subscriber should not receive event from removed topic")
	default:
	}
}

func TestUpdateToEmptyThenUnsubscribeClosesChannel(t *testing.T) {
	b := NewBroker()
	sub := b.Subscribe([]string{"hosts", "advisories"})

	b.Update(sub, nil)
	assert.Empty(t, sub.Topics)

	b.Unsubscribe(sub)

	_, ok := <-sub.Events
	assert.False(t, ok, "channel should be closed after unsubscribe even with empty topics")
}

func TestHostEventConstructors(t *testing.T) {
	assert.Equal(t, Event{Topic: "hosts", Type: "updated"}, NewHostsUpdatedEvent())
	assert.Equal(t, Event{Topic: "host:abc", Type: "snapshot"}, NewHostSnapshotEvent("abc"))
	assert.Equal(t, Event{Topic: "host:abc", Type: "matched"}, NewHostMatchedEvent("abc"))
	assert.Equal(t, Event{Topic: "host:abc", Type: "deleted"}, NewHostDeletedEvent("abc"))
	assert.Equal(t, Event{Topic: "advisories", Type: "updated"}, NewAdvisoriesUpdatedEvent())
}

func TestNonBlockingPublish(t *testing.T) {
	b := NewBroker()
	sub := b.Subscribe([]string{"hosts"})
	defer b.Unsubscribe(sub)

	for i := 0; i < subscriberBufferSize+10; i++ {
		b.Publish(NewHostsUpdatedEvent())
	}

	received := 0
	for {
		select {
		case <-sub.Events:
			received++
		default:
			assert.LessOrEqual(t, received, subscriberBufferSize,
				"should not receive more than buffer size")
			return
		}
	}
}

func TestSharedChannelAcrossTopics(t *testing.T) {
	b := NewBroker()
	sub := b.Subscribe([]string{"hosts", "advisories"})
	defer b.Unsubscribe(sub)

	b.Publish(NewHostsUpdatedEvent())
	b.Publish(NewAdvisoriesUpdatedEvent())

	evs := make([]Event, 0, 2)
	for {
		select {
		case ev := <-sub.Events:
			evs = append(evs, ev)
		default:
			require.Len(t, evs, 2, "should receive both events on same channel")
			assert.Equal(t, "hosts", evs[0].Topic)
			assert.Equal(t, "advisories", evs[1].Topic)
			return
		}
	}
}
