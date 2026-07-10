# Install from RPM packages

For production deployments on RPM-based systems (Rocky Linux, AlmaLinux), you can install PatchBase from our package repository.

## Set up the repository

Create the repo file:

```bash
sudo tee /etc/yum.repos.d/patchbase.repo << 'EOF'
[patchbase]
name=Patchbase
baseurl=https://packages.patchbase.net/rpm/el/$releasever/$basearch/
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.patchbase.net/keys/patchbase.asc
EOF
```

## Install the server

```bash
sudo dnf install patchbase-server
```

This installs:

- The `patchbase-server` binary at `/usr/bin/patchbase-server`
- A systemd service at `/usr/lib/systemd/system/patchbase-server.service`
- A sample config at `/etc/patchbase-server/config.example.yaml`

## Configure the server

```bash
sudo cp /etc/patchbase-server/config.example.yaml /etc/patchbase-server/config.yaml
sudo vi /etc/patchbase-server/config.yaml
```

At minimum, set the following:

```yaml
encryption_key: "<openssl rand -hex 32>"

api:
  jwt_secret_key: "<openssl rand -hex 32>"

database:
  url: "postgres://user:password@localhost:5432/patchbase?sslmode=disable"
```

Make sure PostgreSQL is running and the `patchbase` database exists before proceeding.

## Run migrations

```bash
sudo patchbase-server migrate
```

## Start the service

```bash
sudo systemctl enable --now patchbase-server
```

Check it's running:

```bash
sudo systemctl status patchbase-server
```

The server should now be listening on port 5199. If you have a firewall, open it:

```bash
sudo firewall-cmd --add-port=5199/tcp --permanent
sudo firewall-cmd --reload
```

## Install the agent

On each host you want to monitor:

```bash
sudo dnf install patchbase-agent
```

Then enroll the agent with a registration token you create from the dashboard:

```bash
sudo patchbase-agent enroll http://<server-ip>:5199 pb_reg_<your-token>
```

See the [agent onboarding guide](../onboarding/agent-mode) for full instructions.