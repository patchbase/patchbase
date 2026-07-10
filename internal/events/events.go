package events

const (
	topicHosts      = "hosts"
	topicAdvisories = "advisories"
)

func hostTopic(hostID string) string {
	return "host:" + hostID
}

func NewHostsUpdatedEvent() Event {
	return Event{Topic: topicHosts, Type: "updated", Data: nil}
}

func NewHostSnapshotEvent(hostID string) Event {
	return Event{Topic: hostTopic(hostID), Type: "snapshot", Data: nil}
}

func NewHostMatchedEvent(hostID string) Event {
	return Event{Topic: hostTopic(hostID), Type: "matched", Data: nil}
}

func NewHostDeletedEvent(hostID string) Event {
	return Event{Topic: hostTopic(hostID), Type: "deleted", Data: nil}
}

func NewAdvisoriesUpdatedEvent() Event {
	return Event{Topic: topicAdvisories, Type: "updated", Data: nil}
}
