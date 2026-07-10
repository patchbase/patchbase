package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/events"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
)

const (
	authTimeout    = 10 * time.Second
	pingInterval   = 30 * time.Second
	debounceWindow = 50 * time.Millisecond
	writeTimeout   = 10 * time.Second
)

// Hub manages websocket connections: upgrade, auth, subscription,
// and the broker->client message pump.
type Hub interface {
	HandleWS(w http.ResponseWriter, r *http.Request)
}

type localHub struct {
	broker     events.Broker
	auth       services.Auth
	hosts      services.Hosts
	advisories services.AdvisorySyncService
	logger     *slog.Logger
}

func NewHub(i do.Injector) (Hub, error) {
	broker, err := do.Invoke[events.Broker](i)
	if err != nil {
		return nil, err
	}
	authService, err := do.Invoke[services.Auth](i)
	if err != nil {
		return nil, err
	}
	hostsSvc, err := do.Invoke[services.Hosts](i)
	if err != nil {
		return nil, err
	}
	advSvc, err := do.Invoke[services.AdvisorySyncService](i)
	if err != nil {
		return nil, err
	}
	logger, err := do.Invoke[*slog.Logger](i)
	if err != nil {
		return nil, err
	}
	return &localHub{
		broker:     broker,
		auth:       authService,
		hosts:      hostsSvc,
		advisories: advSvc,
		logger:     logger.With("source", "WSHub"),
	}, nil
}

type clientMessage struct {
	Type   string   `json:"type"`
	Token  string   `json:"token,omitempty"`
	Topics []string `json:"topics,omitempty"`
}

type serverMessage struct {
	Type    string `json:"type"`
	HostID  string `json:"host_id,omitempty"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

type advisoriesData struct {
	Scopes   any `json:"scopes"`
	Overview any `json:"overview"`
}

type wsClient struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

type activeTopics struct {
	mu     sync.RWMutex
	topics map[string]struct{}
}

func newActiveTopics(initial []string) *activeTopics {
	m := make(map[string]struct{}, len(initial))
	for _, t := range initial {
		m[t] = struct{}{}
	}
	return &activeTopics{topics: m}
}

func (a *activeTopics) has(topic string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.topics[topic]
	return ok
}

func (a *activeTopics) add(topics []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, t := range topics {
		a.topics[t] = struct{}{}
	}
}

func (a *activeTopics) remove(topics []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, t := range topics {
		delete(a.topics, t)
	}
}

func (c *wsClient) writeJSON(ctx context.Context, msg serverMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return c.conn.Write(writeCtx, websocket.MessageText, data)
}

func (h *localHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		h.logger.Warn("ws accept failed", "error", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	client := &wsClient{conn: conn}

	// read auth message
	authCtx, authCancel := context.WithTimeout(ctx, authTimeout)
	user, err := h.readAuth(authCtx, conn)
	authCancel()
	if err != nil {
		_ = client.writeJSON(ctx, serverMessage{Type: "error", Message: "unauthorized"})
		conn.Close(websocket.StatusPolicyViolation, "unauthorized")
		return
	}

	_ = client.writeJSON(ctx, serverMessage{Type: "auth_ok"})

	// read subscription message
	subCtx, subCancel := context.WithTimeout(ctx, authTimeout)
	topics, err := h.readSubscription(subCtx, conn)
	subCancel()
	if err != nil {
		h.logger.Debug("ws subscription read failed", "user", user.Email, "error", err)
		return
	}

	sub := h.broker.Subscribe(topics)
	defer h.broker.Unsubscribe(sub)

	h.logger.Info("ws client connected", "user", user.Email, "topics", topics)

	h.pump(ctx, client, sub)
}

func (h *localHub) readAuth(ctx context.Context, conn *websocket.Conn) (sql.User, error) {
	_, data, err := conn.Read(ctx)
	if err != nil {
		return sql.User{}, err
	}
	var msg clientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return sql.User{}, err
	}
	if msg.Type != "auth" || strings.TrimSpace(msg.Token) == "" {
		return sql.User{}, errInvalidAuthMessage
	}
	return h.auth.Authenticate(ctx, msg.Token)
}

func (h *localHub) readSubscription(ctx context.Context, conn *websocket.Conn) ([]string, error) {
	_, data, err := conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	var msg clientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	if msg.Type != "subscribe" || len(msg.Topics) == 0 {
		return nil, errInvalidSubscriptionMessage
	}
	return msg.Topics, nil
}

func (h *localHub) pump(ctx context.Context, client *wsClient, sub *events.Subscriber) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	active := newActiveTopics(sub.Topics)

	done := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		h.readLoop(ctx, client, sub, active, done)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		defer close(done)
		h.eventLoop(ctx, client, sub, active)
	}()

	wg.Wait()
}

func (h *localHub) eventLoop(ctx context.Context, client *wsClient, sub *events.Subscriber, active *activeTopics) {
	debouncers := make(map[string]*time.Timer)
	var mu sync.Mutex

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	// Stop all debouncers on exit.
	defer func() {
		mu.Lock()
		for _, t := range debouncers {
			t.Stop()
		}
		mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-sub.Events:
			if !ok {
				return
			}
			h.handleEvent(ctx, client, ev, &mu, debouncers, active)
		case <-ticker.C:
			if err := client.writeJSON(ctx, serverMessage{Type: "ping"}); err != nil {
				h.logger.Debug("ws ping failed, disconnecting", "error", err)
				return
			}
		}
	}
}

func (h *localHub) handleEvent(
	ctx context.Context,
	client *wsClient,
	ev events.Event,
	mu *sync.Mutex,
	debouncers map[string]*time.Timer,
	active *activeTopics,
) {
	// Drop events for topics the client has unsubscribed from.
	if !active.has(ev.Topic) {
		return
	}

	// Notification-only topics (host:{id}) are sent immediately.
	// Full-data topics (hosts, advisories) are debounced.
	switch ev.Topic {
	case "hosts":
		mu.Lock()
		if timer, exists := debouncers[ev.Topic]; exists {
			timer.Reset(debounceWindow)
		} else {
			debouncers[ev.Topic] = time.AfterFunc(debounceWindow, func() {
				mu.Lock()
				delete(debouncers, ev.Topic)
				mu.Unlock()
				if active.has(ev.Topic) {
					h.pushHosts(ctx, client)
				}
			})
		}
		mu.Unlock()
	case "advisories":
		mu.Lock()
		if timer, exists := debouncers[ev.Topic]; exists {
			timer.Reset(debounceWindow)
		} else {
			debouncers[ev.Topic] = time.AfterFunc(debounceWindow, func() {
				mu.Lock()
				delete(debouncers, ev.Topic)
				mu.Unlock()
				if active.has(ev.Topic) {
					h.pushAdvisories(ctx, client)
				}
			})
		}
		mu.Unlock()
	default:
		// host:{id} topics — send notification immediately.
		hostID := strings.TrimPrefix(ev.Topic, "host:")
		msgType := "host_updated"
		if ev.Type == "deleted" {
			msgType = "host_deleted"
		}
		if err := client.writeJSON(ctx, serverMessage{
			Type:   msgType,
			HostID: hostID,
		}); err != nil {
			h.logger.Debug("ws write failed", "error", err)
		}
	}
}

func (h *localHub) pushHosts(ctx context.Context, client *wsClient) {
	hosts, err := h.hosts.ListHosts(ctx)
	if err != nil {
		h.logger.Warn("ws push hosts fetch failed", "error", err)
		return
	}
	if err := client.writeJSON(ctx, serverMessage{
		Type: "hosts",
		Data: entities.NewHosts(hosts),
	}); err != nil {
		h.logger.Debug("ws write hosts failed", "error", err)
	}
}

func (h *localHub) pushAdvisories(ctx context.Context, client *wsClient) {
	scopes, err := h.advisories.GetScopeStatuses(ctx)
	if err != nil {
		h.logger.Warn("ws push advisories scopes fetch failed", "error", err)
		return
	}
	overview, err := h.advisories.GetOverview(ctx)
	if err != nil {
		h.logger.Warn("ws push advisories overview fetch failed", "error", err)
		return
	}
	if err := client.writeJSON(ctx, serverMessage{
		Type: "advisories",
		Data: advisoriesData{Scopes: scopes, Overview: overview},
	}); err != nil {
		h.logger.Debug("ws write advisories failed", "error", err)
	}
}

func (h *localHub) readLoop(ctx context.Context, client *wsClient, sub *events.Subscriber, active *activeTopics, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		default:
		}
		_, data, err := client.conn.Read(ctx)
		if err != nil {
			return
		}
		var msg clientMessage
		if err := json.Unmarshal(data, &msg); err == nil {
		if (msg.Type == "subscribe" || msg.Type == "unsubscribe") && len(msg.Topics) > 0 {
			if msg.Type == "subscribe" {
				active.add(msg.Topics)
			} else {
				active.remove(msg.Topics)
			}
			newTopics := updateTopics(sub.Topics, msg.Type, msg.Topics)
			h.broker.Update(sub, newTopics)
			h.logger.Debug("ws dynamic topic update", "type", msg.Type, "topics", msg.Topics)
		}
		}
	}
}

func updateTopics(current []string, action string, delta []string) []string {
	m := make(map[string]bool)
	for _, t := range current {
		m[t] = true
	}
	if action == "subscribe" {
		for _, t := range delta {
			m[t] = true
		}
	} else if action == "unsubscribe" {
		for _, t := range delta {
			delete(m, t)
		}
	}
	var res []string
	for t := range m {
		res = append(res, t)
	}
	return res
}
