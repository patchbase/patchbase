# WebSocket events

The dashboard uses a WebSocket connection for real-time updates. The server pushes host and advisory data to connected clients, avoiding the need for polling.

## Connecting

Connect to the WebSocket endpoint at `/api/v1/ws`. No authentication is done via HTTP headers or subprotocols — instead, the client must send an auth message immediately after connecting.

## Client messages

### Authentication

Send a JSON message with `type: "auth"` and your JWT token within 10 seconds of connecting, otherwise the server closes the connection:

```json
{
  "type": "auth",
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

If authentication fails, the server sends:

```json
{
  "type": "error",
  "message": "unauthorized"
}
```

and closes the connection with policy violation status.

### Subscribe

After receiving `auth_ok`, send a subscribe message listing the topics you want to receive:

```json
{
  "type": "subscribe",
  "topics": ["hosts", "advisories", "host:h_xxx"]
}
```

### Dynamic subscription updates

After the initial handshake, the client can send additional `subscribe` and `unsubscribe` messages to change its topic set at any time:

```json
{
  "type": "subscribe",
  "topics": ["host:h_yyy"]
}
```

```json
{
  "type": "unsubscribe",
  "topics": ["host:h_xxx"]
}
```

## Topics

| Topic | Description |
|-------|-------------|
| `hosts` | Aggregate host list — server pushes the full host list (debounced) |
| `advisories` | Advisory scopes and overview — server pushes both (debounced) |
| `host:{hostID}` | Per-host notifications — sent immediately when a host is updated or deleted |

## Server messages

### `auth_ok`

Sent after successful authentication. Confirms the connection is ready for subscriptions.

```json
{
  "type": "auth_ok"
}
```

### `hosts`

Sent when the `hosts` topic is subscribed and host data changes. The `hosts` and `advisories` topics are debounced with a 50ms window, so rapid bursts of events produce a single push. The `data` field contains the full host list:

```json
{
  "type": "hosts",
  "data": [
    {
      "id": "h_xxx",
      "display_name": "web-01",
      "os_name": "Rocky Linux",
      "os_major": 9,
      "overall_action": "none",
      "critical_count": 0,
      "important_count": 2,
      "available_updates": 12,
      "last_seen_at": "2025-01-15T10:00:00Z"
    }
  ]
}
```

### `advisories`

Sent when the `advisories` topic is subscribed and advisory data changes (e.g., after a scope sync). The `data` field contains scope statuses and an overview:

```json
{
  "type": "advisories",
  "data": {
    "scopes": [...],
    "overview": {
      "total_advisories": 5000,
      "total_scopes": 4,
      "synced_scopes": 3
    }
  }
}
```

### `host_updated`

Sent immediately (no debounce) when a specific host receives a new snapshot or is re-matched. Requires subscription to the `host:{hostID}` topic:

```json
{
  "type": "host_updated",
  "host_id": "h_xxx"
}
```

### `host_deleted`

Sent immediately when a host is deleted:

```json
{
  "type": "host_deleted",
  "host_id": "h_xxx"
}
```

### `ping`

The server sends a ping every 30 seconds to keep the connection alive. The client should ignore it — it's just a keepalive message:

```json
{
  "type": "ping"
}
```

### `error`

Sent when authentication fails:

```json
{
  "type": "error",
  "message": "unauthorized"
}
```

## Connection lifecycle

1. Client connects to `/api/v1/ws`
2. Client sends `{"type": "auth", "token": "..."}`
3. Server responds with `{"type": "auth_ok"}`
4. Client sends `{"type": "subscribe", "topics": [...]}`
5. Server pushes messages for subscribed topics as events occur
6. Client can send `subscribe`/`unsubscribe` messages at any time to update topics
7. Server sends `ping` every 30 seconds
8. If a write fails (client disconnected), the server closes the connection