package ws

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActiveTopicsHas(t *testing.T) {
	a := newActiveTopics([]string{"hosts", "advisories"})

	assert.True(t, a.has("hosts"))
	assert.True(t, a.has("advisories"))
	assert.False(t, a.has("host:abc"))
}

func TestActiveTopicsAdd(t *testing.T) {
	a := newActiveTopics([]string{"hosts"})

	a.add([]string{"host:abc", "advisories"})

	assert.True(t, a.has("host:abc"))
	assert.True(t, a.has("advisories"))
	assert.True(t, a.has("hosts"))
}

func TestActiveTopicsRemove(t *testing.T) {
	a := newActiveTopics([]string{"hosts", "advisories", "host:abc"})

	a.remove([]string{"hosts", "host:abc"})

	assert.False(t, a.has("hosts"))
	assert.False(t, a.has("host:abc"))
	assert.True(t, a.has("advisories"))
}

func TestActiveTopicsRemoveNonExistent(t *testing.T) {
	a := newActiveTopics([]string{"hosts"})

	a.remove([]string{"advisories"})

	assert.True(t, a.has("hosts"))
	assert.False(t, a.has("advisories"))
}

func TestUpdateTopicsSubscribe(t *testing.T) {
	result := updateTopics([]string{"hosts", "advisories"}, "subscribe", []string{"host:abc"})

	assert.Contains(t, result, "hosts")
	assert.Contains(t, result, "advisories")
	assert.Contains(t, result, "host:abc")
	assert.Len(t, result, 3)
}

func TestUpdateTopicsUnsubscribe(t *testing.T) {
	result := updateTopics([]string{"hosts", "advisories", "host:abc"}, "unsubscribe", []string{"advisories", "host:abc"})

	assert.Contains(t, result, "hosts")
	assert.NotContains(t, result, "advisories")
	assert.NotContains(t, result, "host:abc")
	assert.Len(t, result, 1)
}

func TestUpdateTopicsUnsubscribeAll(t *testing.T) {
	result := updateTopics([]string{"hosts", "advisories"}, "unsubscribe", []string{"hosts", "advisories"})

	assert.Empty(t, result)
}

func TestUpdateTopicsSubscribeDuplicate(t *testing.T) {
	result := updateTopics([]string{"hosts", "advisories"}, "subscribe", []string{"hosts"})

	assert.Contains(t, result, "hosts")
	assert.Contains(t, result, "advisories")
	assert.Len(t, result, 2)
}

func TestUpdateTopicsEmptyAction(t *testing.T) {
	result := updateTopics([]string{"hosts"}, "unknown", []string{"advisories"})

	assert.Contains(t, result, "hosts")
	assert.NotContains(t, result, "advisories")
}
