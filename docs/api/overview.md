# API overview

PatchBase exposes a REST API under `/api/v1/`. All endpoints return JSON. The API is used by the dashboard, the agent, and any integrations you build.

## Base URL

```
http://<server-host>:5199/api/v1
```

## Endpoints at a glance

### Health

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | None | Health check |

### Setup

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/setup/status` | None | Check if initial setup is needed |
| POST | `/setup/complete` | None | Complete initial setup (create admin) |

### Authentication

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/auth/login` | None | Login with email/password, returns JWT |

### Profile

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/profile` | JWT | Get current user profile |
| PATCH | `/profile` | JWT | Update profile |

### Settings

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/settings` | JWT | Get server settings |
| PATCH | `/settings` | JWT | Update settings |
| POST | `/settings/test-email` | JWT | Send a test email |
| POST | `/settings/send-report` | JWT | Send a vulnerability report email |

### Hosts

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/hosts` | JWT | List all hosts |
| GET | `/hosts/pending` | JWT | List pending (unapproved) hosts |
| GET | `/hosts/tokens` | JWT | List registration tokens |
| POST | `/hosts/tokens` | JWT | Create a registration token |
| POST | `/hosts/tokens/{tokenID}/revoke` | JWT | Revoke a registration token |
| POST | `/hosts/{hostID}/approve` | JWT | Approve a pending host |
| POST | `/hosts/ssh` | JWT | Create an SSH pull host |
| POST | `/hosts/manual` | JWT | Create a manual host |
| GET | `/hosts/manual/script` | JWT | Get the collector script (`?os_family=apt\|rpm`) |
| POST | `/hosts/{hostID}/onboard-ssh` | JWT | Onboard an SSH host |
| POST | `/hosts/{hostID}/report` | JWT | Upload a manual collection report |
| DELETE | `/hosts/{hostID}` | JWT | Delete a host |
| GET | `/hosts/{hostID}` | JWT | Get host details |
| GET | `/hosts/{hostID}/snapshot` | JWT | Get latest snapshot |
| GET | `/hosts/{hostID}/pull-jobs` | JWT | List SSH pull job history |
| POST | `/hosts/{hostID}/pull-now` | JWT | Trigger an immediate SSH pull |
| GET | `/hosts/{hostID}/packages/vulnerable` | JWT | List vulnerable packages |
| GET | `/hosts/{hostID}/packages/upgradable` | JWT | List upgradable packages |
| GET | `/hosts/{hostID}/kernel-posture` | JWT | Get kernel posture info |

### Advisories

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/advisories/scopes` | JWT | List advisory scope statuses |
| POST | `/advisories/scopes/{scopeKey}/sync` | JWT | Trigger manual scope sync |
| GET | `/advisories/overview` | JWT | Get advisory overview |
| GET | `/advisories/{id}` | JWT | Get advisory details |

### Dashboard

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/dashboard/overview` | JWT | Get dashboard summary |

### Agent endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/agent/register` | Registration token | Register a new host |
| POST | `/agent/snapshots` | Host access token | Submit a snapshot |

### WebSocket

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/ws` | JWT | WebSocket for live dashboard updates |

See the individual pages in this section for details on authentication, host endpoints, advisory endpoints, and WebSocket events.