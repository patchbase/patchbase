# Authentication

PatchBase uses two types of authentication: JWT sessions for dashboard/API users and token-based auth for agents.

## JWT session auth

After logging in, the API returns a JWT that must be sent in the `Authorization` header for all protected endpoints:

```
Authorization: Bearer <jwt-token>
```

### Login

```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "your-password"
}
```

Response:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2025-01-15T12:00:00Z"
}
```

The token is signed with the `api.jwt_secret_key` from the server config.

## Agent token auth

Agents authenticate with two different token types:

### Registration tokens

Used once during enrollment. The agent sends it with a `POST /api/v1/agent/register` request:

```http
POST /api/v1/agent/register
Content-Type: application/json

{
  "registration_token": "pb_reg_xxxxxxxx",
  "hostname": "web-01",
  "machine_id": "...",
  "metadata": {
    "ip_address": "10.0.0.5",
    "os_name": "Rocky Linux",
    "os_version": "10.2",
    "architecture": "x86_64"
  }
}
```

The server validates the token, creates a host record, and returns a host access token. Registration tokens can be created and revoked from the dashboard.

### Host access tokens

After enrollment, the agent uses its host access token for all snapshot submissions:

```http
POST /api/v1/agent/snapshots
Authorization: Bearer pb_host_xxxxxxxx
Content-Type: application/x-protobuf

<protobuf body>
```

The token is sent as a Bearer token in the Authorization header. The server validates it against stored token hashes.

## Initial setup

Before any users exist, the setup endpoints are available without authentication:

```http
GET /api/v1/setup/status
```

Returns whether initial setup is needed. If so:

```http
POST /api/v1/setup/complete
```

Creates the first admin user. After setup is complete, these endpoints become unavailable.