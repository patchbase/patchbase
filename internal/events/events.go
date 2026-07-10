package events

const (
	topicHosts      = "hosts"
	topicAdvisories = "advisories"
)

func hostTopic(hostID string) string {
	return "host:" + hostID
}

func NewHostsUpdatedEvent() Event {
	return Event{Topic: topicHosts, Type: "updated"}
}

func NewHostSnapshotEvent(hostID string) Event {
	return Event{Topic: hostTopic(hostID), Type: "snapshot"}
}

func NewHostMatchedEvent(hostID string) Event {
	return Event{Topic: hostTopic(hostID), Type: "matched"}
}

func NewHostDeletedEvent(hostID string) Event {
	return Event{Topic: hostTopic(hostID), Type: "deleted"}
}

func NewAdvisoriesUpdatedEvent() Event {
	return Event{Topic: topicAdvisories, Type: "updated"}
}
