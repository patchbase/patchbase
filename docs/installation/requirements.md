# Requirements

## Server

The PatchBase server runs on any modern Linux system. Here's what you'll need:

### Operating system

Any Linux distribution. We provide packages for:

**RPM-based:**
- Rocky Linux 9 / 10
- AlmaLinux 9 / 10

**DEB-based:**
- Ubuntu 22.04 / 24.04 / 26.04
- Debian 12 / 13

You can also [build from source](./build-from-source) or run via Docker Compose.

### PostgreSQL

PatchBase requires **PostgreSQL 13 or newer**. We develop and test against PostgreSQL 16, but anything from 13 onward works fine. You'll need:

- A dedicated database (e.g., `patchbase`)
- A user with full permissions on that database

If you don't already have PostgreSQL running, the [quick start guide](./quickstart) shows how to spin one up with Docker Compose.

### System resources

The server is lightweight. For a small fleet (up to ~50 hosts):

- **CPU:** 1 vCPU
- **RAM:** 512 MB
- **Disk:** 1 GB for the application and advisory databases

Larger deployments will benefit from more RAM and CPU, but PatchBase is not resource-hungry.

### Network

The server listens on port **5199** by default. If you're running behind a reverse proxy, make sure WebSocket connections are forwarded (the dashboard uses WebSockets for live updates).

## Agent

The agent runs on each host you want to monitor. It's a single static binary with no runtime dependencies.

### Supported hosts

| OS family | Distributions |
|-----------|--------------|
| APT | Ubuntu 22.04+, Debian 12+ |
| RPM | Rocky Linux 9+, AlmaLinux 9+ |

The agent needs:

- **Outbound network access** to the PatchBase server (HTTP/HTTPS on port 5199 by default)
- **Read access** to `/etc/os-release`, `/etc/machine-id`, and the package manager's database

No root privileges are strictly required for collection, but the systemd timer is typically run as root.

## SSH pull mode

If you prefer not to install the agent on each host, the server can SSH into your hosts directly. For this mode you'll need:

- SSH access from the server to each host
- An SSH key pair (PatchBase can generate one per host, or use a shared key)
- `bash` and basic utilities (`hostname`, `uname`, `dpkg-query` or `rpm`) on the target host