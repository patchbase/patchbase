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
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/events"
	"go.patchbase.net/server/internal/mock"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
	"go.uber.org/mock/gomock"
)

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

const (
	testUserID      = "u1"
	testUserEmail   = "test@example.com"
	testUserName    = "Test"
	testHostID      = "h1"
	testHostName    = "host1"
	testToken       = "test-token"
	testScopeKey    = "el9"
	testTotalAdv    = int64(42)
	testTotalScopes = int32(1)
	testSynced      = int32(1)
)

func newTestHub(t *testing.T, broker events.Broker) *localHub {
	t.Helper()
	ctrl := gomock.NewController(t)

	auth := mock.NewMockAuth(ctrl)
	auth.EXPECT().Authenticate(gomock.Any(), testToken).
		Return(sql.User{ID: testUserID, Email: testUserEmail, Name: testUserName}, nil).
		AnyTimes()
	auth.EXPECT().Authenticate(gomock.Any(), gomock.Not(testToken)).
		Return(sql.User{}, apperr.ErrUnauthorized).
		AnyTimes()

	hosts := mock.NewMockHosts(ctrl)
	hosts.EXPECT().ListHosts(gomock.Any()).
		Return([]services.HostInfo{{ID: testHostID, DisplayName: testHostName}}, nil).
		AnyTimes()

	advisories := mock.NewMockAdvisorySyncService(ctrl)
	advisories.EXPECT().GetScopeStatuses(gomock.Any()).
		Return([]services.AdvisoryScopeStatus{{ScopeKey: testScopeKey}}, nil).
		AnyTimes()
	advisories.EXPECT().GetOverview(gomock.Any()).
		Return(services.AdvisoryOverview{
			TotalAdvisories: testTotalAdv,
			TotalScopes:     testTotalScopes,
			SyncedScopes:    testSynced,
		}, nil).
		AnyTimes()

	return &localHub{
		broker:     broker,
		auth:       auth,
		hosts:      hosts,
		advisories: advisories,
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
		mustJSON(t, clientMessage{Type: "auth", Token: testToken})))

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
	hub := newTestHub(t, broker)

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
	hub := newTestHub(t, broker)

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
	hub := newTestHub(t, broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn := dialAndAuth(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")
	sendSubscribe(t, conn, broker, "hosts")

	broker.Publish(events.NewHostsUpdatedEvent())

	msg, raw := readServerMessageRaw(t, conn)
	assert.Equal(t, "hosts", msg.Type)

	type hostsPayload struct {
		Type string           `json:"type"`
		Data []map[string]any `json:"data"`
	}
	var payload hostsPayload
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Len(t, payload.Data, 1)
	assert.Equal(t, testHostID, payload.Data[0]["id"])
	assert.Equal(t, testHostName, payload.Data[0]["display_name"])
}

func TestHandleWS_DeliversAdvisoriesPush(t *testing.T) {
	broker := newSignalingBroker()
	hub := newTestHub(t, broker)

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
	assert.Equal(t, testScopeKey, payload.Data.Scopes[0]["scope_key"])
	assert.EqualValues(t, testTotalAdv, payload.Data.Overview["total_advisories"])
	assert.EqualValues(t, testTotalScopes, payload.Data.Overview["total_scopes"])
	assert.EqualValues(t, testSynced, payload.Data.Overview["synced_scopes"])
}

func TestHandleWS_DeliversHostNotification(t *testing.T) {
	broker := newSignalingBroker()
	hub := newTestHub(t, broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn := dialAndAuth(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")
	sendSubscribe(t, conn, broker, "host:"+testHostID)

	broker.Publish(events.NewHostMatchedEvent(testHostID))

	msg := readServerMessage(t, conn)
	assert.Equal(t, "host_updated", msg.Type)
	assert.Equal(t, testHostID, msg.HostID)
}

func TestHandleWS_DeliversHostDeletedNotification(t *testing.T) {
	broker := newSignalingBroker()
	hub := newTestHub(t, broker)

	ts := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer ts.Close()

	conn := dialAndAuth(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")
	sendSubscribe(t, conn, broker, "host:"+testHostID)

	broker.Publish(events.NewHostDeletedEvent(testHostID))

	msg := readServerMessage(t, conn)
	assert.Equal(t, "host_deleted", msg.Type)
	assert.Equal(t, testHostID, msg.HostID)
}

func TestHandleWS_DoesNotDeliverUnsubscribedTopic(t *testing.T) {
	broker := newSignalingBroker()
	hub := newTestHub(t, broker)

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
	hub := newTestHub(t, broker)

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
	hub := newTestHub(t, broker)

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
	hub := newTestHub(t, broker)

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
	sendSubscribe(t, conn, broker, "host:"+testHostID)
	broker.Publish(events.NewHostMatchedEvent(testHostID))

	select {
	case msg := <-msgCh:
		assert.Equal(t, "host_updated", msg.Type, "should receive new topic message")
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for allowed topic")
	}
}
