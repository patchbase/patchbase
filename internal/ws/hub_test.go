// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/events"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
)

type mockAuth struct {
	validToken string
	user       sql.User
}

func (m *mockAuth) Login(_ context.Context, _ string, _ string) (services.LoginResult, error) {
	return services.LoginResult{}, nil
}

func (m *mockAuth) Authenticate(_ context.Context, token string) (sql.User, error) {
	if token == m.validToken {
		return m.user, nil
	}
	return sql.User{}, apperr.ErrUnauthorized
}

func (m *mockAuth) IssueAccessToken(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockAuth) UpdateProfile(_ context.Context, _ string, _ services.UpdateProfileInput) (services.UpdateProfileResult, error) {
	return services.UpdateProfileResult{}, nil
}

var _ services.Auth = (*mockAuth)(nil)

type mockHosts struct {
	hosts []services.HostInfo
}

func (m *mockHosts) ListHosts(_ context.Context) ([]services.HostInfo, error) {
	return m.hosts, nil
}

func (m *mockHosts) GetHost(_ context.Context, _ string) (services.HostInfo, error) {
	return services.HostInfo{}, nil
}

func (m *mockHosts) GetLatestSnapshot(_ context.Context, _ string) (services.HostSnapshotInfo, error) {
	return services.HostSnapshotInfo{}, nil
}

func (m *mockHosts) ListSSHPullJobs(_ context.Context, _ string) ([]services.HostSSHPullJobInfo, error) {
	return nil, nil
}

func (m *mockHosts) RunSSHPull(_ context.Context, _ string) error { return nil }
func (m *mockHosts) CreateRegistrationToken(_ context.Context, _ string, _ string) (services.CreatedRegistrationToken, error) {
	return services.CreatedRegistrationToken{}, nil
}
func (m *mockHosts) ListRegistrationTokens(_ context.Context) ([]services.RegistrationTokenInfo, error) {
	return nil, nil
}
func (m *mockHosts) RevokeRegistrationToken(_ context.Context, _ string) error { return nil }
func (m *mockHosts) RegisterAgentHost(_ context.Context, _ *agentpb.RegisterHostRequest) (*agentpb.RegisterHostResponse, error) {
	return nil, nil
}
func (m *mockHosts) IngestAgentSnapshot(_ context.Context, _ string, _ *agentpb.AgentSnapshot) (*agentpb.SyncResponse, error) {
	return nil, nil
}
func (m *mockHosts) ListPendingHosts(_ context.Context) ([]services.HostInfo, error) {
	return nil, nil
}
func (m *mockHosts) ApproveHost(_ context.Context, _ string) (services.HostInfo, error) {
	return services.HostInfo{}, nil
}
func (m *mockHosts) DeleteHost(_ context.Context, _ string) error { return nil }
func (m *mockHosts) CreateSSHHost(_ context.Context, _ services.CreateSSHHostInput) (services.CreateSSHHostResult, error) {
	return services.CreateSSHHostResult{}, nil
}
func (m *mockHosts) OnboardSSHHost(_ context.Context, _ string) error { return nil }
func (m *mockHosts) GetDashboardOverview(_ context.Context) (services.DashboardOverview, error) {
	return services.DashboardOverview{}, nil
}
func (m *mockHosts) CreateManualHost(_ context.Context, _ string, _ string) (services.HostInfo, error) {
	return services.HostInfo{}, nil
}
func (m *mockHosts) IngestManualReport(_ context.Context, _ string, _ []byte) error { return nil }
func (m *mockHosts) GetCollectorScript(_ string) (string, error) { return "", nil }

var _ services.Hosts = (*mockHosts)(nil)

type mockAdvisories struct {
	scopes   []services.AdvisoryScopeStatus
	overview services.AdvisoryOverview
}

func (m *mockAdvisories) GetScopeStatuses(_ context.Context) ([]services.AdvisoryScopeStatus, error) {
	return m.scopes, nil
}

func (m *mockAdvisories) GetOverview(_ context.Context) (services.AdvisoryOverview, error) {
	return m.overview, nil
}

func (m *mockAdvisories) SyncScope(_ context.Context, _ string) error                { return nil }
func (m *mockAdvisories) TriggerManualSync(_ context.Context, _ string) error         { return nil }
func (m *mockAdvisories) ResolveScopeKey(_ context.Context, _, _, _ string, _ int32, _ string) (string, error) {
	return "", nil
}
func (m *mockAdvisories) RegisterScopeDemand(_ context.Context, _ string) error { return nil }
func (m *mockAdvisories) GetAdvisory(_ context.Context, _ string) (sql.Advisory, error) {
	return sql.Advisory{}, nil
}

var _ services.AdvisorySyncService = (*mockAdvisories)(nil)

// signalingBroker wraps events.Broker and signals a channel after
// each Subscribe or Update call completes, so tests can synchronize
// without sleeps.
type signalingBroker struct {
	events.Broker
	updateCh chan struct{}
}

func newSignalingBroker() *signalingBroker {
	return &signalingBroker{
		Broker:   events.NewBroker(),
		updateCh: make(chan struct{}, 64),
	}
}

func (s *signalingBroker) Subscribe(topics []string) *events.Subscriber {
	sub := s.Broker.Subscribe(topics)
	s.updateCh <- struct{}{}
	return sub
}

func (s *signalingBroker) Update(sub *events.Subscriber, topics []string) {
	s.Broker.Update(sub, topics)
	s.updateCh <- struct{}{}
}

func (s *signalingBroker) waitForSignal(t *testing.T) {
	t.Helper()
	select {
	case <-s.updateCh:
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for broker signal")
	}
}

func newTestHub(broker events.Broker) *localHub {
	return &localHub{
		broker: broker,
		auth: &mockAuth{
			validToken: "test-token",
			user:       sql.User{ID: "u1", Email: "test@example.com", Name: "Test"},
		},
		hosts:      &mockHosts{hosts: []services.HostInfo{{ID: "h1", DisplayName: "host1"}}},
		advisories: &mockAdvisories{scopes: []services.AdvisoryScopeStatus{{ScopeKey: "el9"}}, overview: services.AdvisoryOverview{TotalAdvisories: 42, TotalScopes: 1, SyncedScopes: 1}},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func wsURL(ts *httptest.Server) string {
	return "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/ws"
}

func dialAndAuth(t *testing.T, ts *httptest.Server) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.Dial(context.Background(), wsURL(ts), nil)
	require.NoError(t, err)

	require.NoError(t, conn.Write(context.Background(), websocket.MessageText,
		mustJSON(t, clientMessage{Type: "auth", Token: "test-token"})))

	msg := readServerMessage(t, conn)
	require.Equal(t, "auth_ok", msg.Type)
	return conn
}

func sendSubscribe(t *testing.T, conn *websocket.Conn, broker *signalingBroker, topics ...string) {
	t.Helper()
	require.NoError(t, conn.Write(context.Background(), websocket.MessageText,
		mustJSON(t, clientMessage{Type: "subscribe", Topics: topics})))
	broker.waitForSignal(t)
}

func sendUnsubscribe(t *testing.T, conn *websocket.Conn, broker *signalingBroker, topics ...string) {
	t.Helper()
	require.NoError(t, conn.Write(context.Background(), websocket.MessageText,
		mustJSON(t, clientMessage{Type: "unsubscribe", Topics: topics})))
	broker.waitForSignal(t)
}

func readServerMessage(t *testing.T, conn *websocket.Conn) serverMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	require.NoError(t, err)
	var msg serverMessage
	require.NoError(t, json.Unmarshal(data, &msg))
	return msg
}

func readServerMessageRaw(t *testing.T, conn *websocket.Conn) (serverMessage, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	require.NoError(t, err)
	var msg serverMessage
	require.NoError(t, json.Unmarshal(data, &msg))
	return msg, data
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

func TestHandleWS_RejectsInvalidAuth(t *testing.T) {
	broker := events.NewBroker()
	hub := newTestHub(broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn, _, err := websocket.Dial(context.Background(), wsURL(ts), nil)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	require.NoError(t, conn.Write(context.Background(), websocket.MessageText,
		mustJSON(t, clientMessage{Type: "auth", Token: "wrong"})))

	msg := readServerMessage(t, conn)
	assert.Equal(t, "error", msg.Type)
	assert.Equal(t, "unauthorized", msg.Message)
}

func TestHandleWS_RejectsMissingAuth(t *testing.T) {
	broker := events.NewBroker()
	hub := newTestHub(broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn, _, err := websocket.Dial(context.Background(), wsURL(ts), nil)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	require.NoError(t, conn.Write(context.Background(), websocket.MessageText,
		mustJSON(t, clientMessage{Type: "subscribe", Topics: []string{"hosts"}})))

	msg := readServerMessage(t, conn)
	assert.Equal(t, "error", msg.Type)
	assert.Equal(t, "unauthorized", msg.Message)
}

func TestHandleWS_DeliversHostsPush(t *testing.T) {
	broker := newSignalingBroker()
	hub := newTestHub(broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn := dialAndAuth(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")
	sendSubscribe(t, conn, broker, "hosts")

	broker.Publish(events.NewHostsUpdatedEvent())

	msg, raw := readServerMessageRaw(t, conn)
	assert.Equal(t, "hosts", msg.Type)

	type hostsPayload struct {
		Type string `json:"type"`
		Data []map[string]any `json:"data"`
	}
	var payload hostsPayload
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Len(t, payload.Data, 1)
	assert.Equal(t, "h1", payload.Data[0]["id"])
	assert.Equal(t, "host1", payload.Data[0]["display_name"])
}

func TestHandleWS_DeliversAdvisoriesPush(t *testing.T) {
	broker := newSignalingBroker()
	hub := newTestHub(broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn := dialAndAuth(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")
	sendSubscribe(t, conn, broker, "advisories")

	broker.Publish(events.NewAdvisoriesUpdatedEvent())

	msg, raw := readServerMessageRaw(t, conn)
	assert.Equal(t, "advisories", msg.Type)

	type advisoriesPayload struct {
		Type string `json:"type"`
		Data struct {
			Scopes   []map[string]any `json:"scopes"`
			Overview map[string]any   `json:"overview"`
		} `json:"data"`
	}
	var payload advisoriesPayload
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Len(t, payload.Data.Scopes, 1)
	assert.Equal(t, "el9", payload.Data.Scopes[0]["scope_key"])
	assert.EqualValues(t, 42, payload.Data.Overview["total_advisories"])
	assert.EqualValues(t, 1, payload.Data.Overview["total_scopes"])
	assert.EqualValues(t, 1, payload.Data.Overview["synced_scopes"])
}

func TestHandleWS_DeliversHostNotification(t *testing.T) {
	broker := newSignalingBroker()
	hub := newTestHub(broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn := dialAndAuth(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")
	sendSubscribe(t, conn, broker, "host:h1")

	broker.Publish(events.NewHostMatchedEvent("h1"))

	msg := readServerMessage(t, conn)
	assert.Equal(t, "host_updated", msg.Type)
	assert.Equal(t, "h1", msg.HostID)
}

func TestHandleWS_DeliversHostDeletedNotification(t *testing.T) {
	broker := newSignalingBroker()
	hub := newTestHub(broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn := dialAndAuth(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")
	sendSubscribe(t, conn, broker, "host:h1")

	broker.Publish(events.NewHostDeletedEvent("h1"))

	msg := readServerMessage(t, conn)
	assert.Equal(t, "host_deleted", msg.Type)
	assert.Equal(t, "h1", msg.HostID)
}

func TestHandleWS_DoesNotDeliverUnsubscribedTopic(t *testing.T) {
	broker := newSignalingBroker()
	hub := newTestHub(broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn := dialAndAuth(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")
	sendSubscribe(t, conn, broker, "hosts", "advisories")

	sendUnsubscribe(t, conn, broker, "advisories")

	broker.Publish(events.NewAdvisoriesUpdatedEvent())

	msgCh := make(chan serverMessage, 1)
	go func() {
		ctx := context.Background()
		_, data, err := conn.Read(ctx)
		if err == nil {
			var msg serverMessage
			_ = json.Unmarshal(data, &msg)
			msgCh <- msg
		}
	}()

	select {
	case msg := <-msgCh:
		require.Fail(t, "should not receive message for unsubscribed topic", "got: %v", msg)
	case <-time.After(100 * time.Millisecond):
		// Passed debounce window
	}

	broker.Publish(events.NewHostsUpdatedEvent())

	select {
	case msg := <-msgCh:
		assert.Equal(t, "hosts", msg.Type, "should receive hosts")
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for allowed topic")
	}
}

func TestHandleWS_DynamicSubscribeReceivesEvents(t *testing.T) {
	broker := newSignalingBroker()
	hub := newTestHub(broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn := dialAndAuth(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")
	sendSubscribe(t, conn, broker, "hosts")

	sendSubscribe(t, conn, broker, "advisories")

	broker.Publish(events.NewAdvisoriesUpdatedEvent())

	msg := readServerMessage(t, conn)
	assert.Equal(t, "advisories", msg.Type)
}

func TestHandleWS_PartialUnsubscribeStopsDelivery(t *testing.T) {
	broker := newSignalingBroker()
	hub := newTestHub(broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn := dialAndAuth(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")
	sendSubscribe(t, conn, broker, "hosts", "advisories")

	sendUnsubscribe(t, conn, broker, "hosts")

	broker.Publish(events.NewHostsUpdatedEvent())

	msgCh := make(chan serverMessage, 1)
	go func() {
		ctx := context.Background()
		_, data, err := conn.Read(ctx)
		if err == nil {
			var msg serverMessage
			_ = json.Unmarshal(data, &msg)
			msgCh <- msg
		}
	}()

	select {
	case msg := <-msgCh:
		require.Fail(t, "should not receive message for unsubscribed topic", "got: %v", msg)
	case <-time.After(100 * time.Millisecond):
		// Passed debounce window
	}

	broker.Publish(events.NewAdvisoriesUpdatedEvent())

	select {
	case msg := <-msgCh:
		assert.Equal(t, "advisories", msg.Type, "should receive advisories")
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for allowed topic")
	}
}

func TestHandleWS_UnsubscribeAllStopsDelivery(t *testing.T) {
	broker := newSignalingBroker()
	hub := newTestHub(broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn := dialAndAuth(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")
	sendSubscribe(t, conn, broker, "hosts", "advisories")

	sendUnsubscribe(t, conn, broker, "hosts", "advisories")

	broker.Publish(events.NewHostsUpdatedEvent())
	broker.Publish(events.NewAdvisoriesUpdatedEvent())

	msgCh := make(chan serverMessage, 1)
	go func() {
		ctx := context.Background()
		_, data, err := conn.Read(ctx)
		if err == nil {
			var msg serverMessage
			_ = json.Unmarshal(data, &msg)
			msgCh <- msg
		}
	}()

	select {
	case msg := <-msgCh:
		require.Fail(t, "should not receive message for unsubscribed topic", "got: %v", msg)
	case <-time.After(100 * time.Millisecond):
		// Passed debounce window
	}

	// Prove connection liveness
	sendSubscribe(t, conn, broker, "host:h1")
	broker.Publish(events.NewHostMatchedEvent("h1"))

	select {
	case msg := <-msgCh:
		assert.Equal(t, "host_updated", msg.Type, "should receive new topic message")
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for allowed topic")
	}
}
