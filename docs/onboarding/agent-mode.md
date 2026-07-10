# Agent mode

The agent is the most hands-off way to onboard hosts. Install it once, enroll it with a token, and the systemd timer takes care of the rest.

## Step 1: Create a registration token

In the PatchBase dashboard, go to **Hosts → Register** and create a new registration token. You'll get a token string that looks like `pb_reg_<random>`.

Copy it — you'll use it on the host.

## Step 2: Install the agent

If you've [set up the RPM repo](../installation/rpm-packages):

```bash
sudo dnf install patchbase-agent
```

Or if you [built from source](../installation/build-from-source), copy the binary to the host:

```bash
sudo cp patchbase-agent /usr/bin/patchbase-agent
```

## Step 3: Enroll the agent

Run the enroll command, passing the server URL and your registration token:

```bash
sudo patchbase-agent enroll http://<server-ip>:5199 pb_reg_<your-token>
```

This does a few things:

1. Contacts the server and registers the host
2. Receives a host access token back
3. Writes a config file to `/etc/patchbase-agent/config.json`

You'll see output like:

```
Successfully enrolled host
  config_path=/etc/patchbase-agent/config.json
  server_url=http://server:5199
  host_id=h_xxxxxxxxxxxx
  approval_status=pending
```

## Step 4: Approve the host

Back in the dashboard, go to **Hosts → Pending**. You'll see your new host waiting for approval. Click **Approve** to let it start sending snapshots.

The host won't sync until it's approved. This is a safety measure — you control what gets into your PatchBase instance.

## Step 5: Enable the systemd timer

If you installed via RPM, the timer is already installed. Enable it:

```bash
sudo systemctl enable --now patchbase-agent.timer
```

The timer runs `patchbase-agent sync` every 10 minutes by default (2 minutes after boot, then every 10 minutes). The agent collects a snapshot and sends it to the server.

If you installed the binary manually, you'll need to set up the systemd unit files yourself. See the [packaging files](https://github.com/patchbase/patchbase/tree/main/packaging) for reference.

## Verifying it works

After a few minutes, the host should appear in the dashboard with its OS details, package count, and available updates. You can also trigger a manual sync:

```bash
sudo patchbase-agent sync
```

Add `--debug` to print the snapshot JSON to stdout without sending it:

```bash
sudo patchbase-agent sync --debug
```

## Using a custom CA certificate

If your server uses a self-signed certificate or a private CA, point the agent at the CA bundle:

```bash
sudo patchbase-agent enroll https://server:5199 pb_reg_token --ca-cert /path/to/ca.pem
```

For development with plain HTTP, pass `-k`:

```bash
sudo patchbase-agent enroll http://server:5199 pb_reg_token --allow-insecure-http
```

## What the agent collects

Each snapshot includes:

- **Host metadata** — hostname, machine ID, OS name and version, architecture, kernel version, boot time, uptime
- **Installed packages** — name, version, architecture, vendor, source package
- **Enabled repositories** — repo ID, base URL, enabled status
- **Available updates** — packages with newer versions available in enabled repos

The agent does not collect process data, network connections, or file contents beyond what's listed above.