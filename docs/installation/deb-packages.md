# Install from DEB packages

For production deployments on DEB-based systems (Ubuntu, Debian), you can install PatchBase from our APT repository.

## Set up the repository

```bash
# Add the PatchBase APT repository
curl -fsSL https://packages.patchbase.net/keys/patchbase.asc | sudo gpg --dearmor -o /usr/share/keyrings/patchbase.gpg

echo "deb [signed-by=/usr/share/keyrings/patchbase.gpg] https://packages.patchbase.net/deb stable main" | sudo tee /etc/apt/sources.list.d/patchbase.list

sudo apt update
```

## Install the server

```bash
sudo apt install patchbase-server
```

This installs:

- The `patchbase-server` binary at `/usr/bin/patchbase-server`
- A systemd service at `/usr/lib/systemd/system/patchbase-server.service`
- A sample config at `/etc/patchbase-server/config.example.yaml`

## Configure the server

```bash
sudo cp /etc/patchbase-server/config.example.yaml /etc/patchbase-server/config.yaml
sudo nano /etc/patchbase-server/config.yaml
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

The server should now be listening on port 5199. If you have a firewall (UFW), open it:

```bash
sudo ufw allow 5199/tcp
```

## Install the agent

On each host you want to monitor:

```bash
sudo apt install patchbase-agent
```

Then enroll the agent with a registration token you create from the dashboard:

```bash
sudo patchbase-agent enroll http://<server-ip>:5199 pb_reg_<your-token>
```

See the [agent onboarding guide](../onboarding/agent-mode) for full instructions.