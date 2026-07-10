# SSH pull mode

SSH pull mode lets the PatchBase server collect snapshots from your hosts over SSH — no agent installation required. The server connects on a schedule, runs a collection script, parses the output, and stores the result.

## How it works

1. You create an SSH host in the dashboard, providing the hostname and SSH user.
2. PatchBase generates an SSH key pair (either per-host or a shared global key).
3. You install the public key on the target host.
4. The server SSHes in on a schedule, detects the OS family (APT or RPM), runs the appropriate collector script, and parses the output into a snapshot.

## Step 1: Create an SSH host

In the dashboard, go to **Hosts → Register** and choose **SSH Pull**. Fill in:

- **Display name** — a friendly label for the host
- **Hostname** — the hostname or IP address the server will connect to
- **SSH user** — the user the server will SSH as (needs read access to package databases)
- **Polling frequency** — how often to collect, in minutes (default: 360, i.e., every 6 hours)

You can choose to use a **unique key pair** for this host or the **global SSH key**. If you pick the global key, make sure you've set one up in Settings first.

## Step 2: Install the public key

After creating the host, you'll see the public key in the dashboard. Copy it and add it to the target host:

```bash
mkdir -p ~/.ssh
echo "ssh-ed25519 AAAA..." >> ~/.ssh/authorized_keys
chmod 700 ~/.ssh
chmod 600 ~/.ssh/authorized_keys
```

The SSH user you specified needs to be able to run:

- `hostname`, `uname`, `cat` — basic system info
- `dpkg-query` (APT) or `rpm` (RPM) — package listing
- `apt list --upgradable` (APT) or `dnf check-update` / `yum check-update` (RPM) — available updates

No root access is required as long as the user can read the package databases.

## Step 3: Onboard the host

Click **Onboard** in the dashboard. This marks the host as ready and schedules the first SSH pull job.

If the connection succeeds, the host will show up with its OS details and package inventory within a minute or two. If it fails, check the error message in the **Pull jobs** history on the host detail page.

## Step 4: Verify

Go to the host detail page. You should see:

- OS family, name, version, and architecture
- Package count and available updates
- Running kernel
- Last pull job status and timestamp

You can also trigger a pull manually with the **Pull now** button.

## Troubleshooting SSH connections

**Connection refused / timeout**

Check that the host is reachable from the server and that SSH (port 22) is open:

```bash
# Run from the PatchBase server
ssh -i /path/to/private/key <user>@<hostname>
```

**Permission denied (publickey)**

The public key hasn't been installed correctly on the target. Verify it's in `~/.ssh/authorized_keys` and that file permissions are correct (700 for `~/.ssh`, 600 for `authorized_keys`).

**Host key verification failed**

PatchBase's SSH client skips host key verification during pull jobs. If you're seeing this, make sure you're running a recent build.

**Wrong OS detected**

The server runs a tiny script that reads `/etc/os-release` to detect the OS family. If your distribution's `os-release` file uses unusual `ID` or `ID_LIKE` values, the detection might fail. File an issue and we'll add support for your distribution.