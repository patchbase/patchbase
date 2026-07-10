# Manual mode

Manual mode is the simplest way to get a host's data into PatchBase — no agent, no SSH connection from the server. You download a collection script, run it on the host, and upload the result.

This is useful for air-gapped systems, one-off assessments, or environments where neither agent installation nor server-initiated SSH is possible.

## Step 1: Create a manual host

In the dashboard, go to **Hosts → Register** and choose **Manual**. Enter a display name and hostname. The host is created immediately — no approval step needed for manual hosts.

## Step 2: Download the collector script

On the host detail page, you'll see a **Download collector script** button. The script is tailored to the OS family you select:

- **APT** — for Debian/Ubuntu systems (uses `dpkg-query`)
- **RPM** — for Rocky/AlmaLinux systems (uses `rpm`)

Choose the right one for the host you're collecting from, then download the script.

## Step 3: Run the script on the host

Transfer the script to the target host (via USB, SCP, copy-paste, etc.) and run it:

```bash
chmod +x patchbase-collect.sh
./patchbase-collect.sh > report.txt
```

The script collects:

- Hostname, machine ID, architecture, kernel version
- OS name, version, and family
- Installed packages (name, version, architecture, vendor, source package)
- Enabled repositories
- Available package updates

The output is plain text with metadata headers and delimited sections. It doesn't contain any secrets — just a package inventory.

## Step 4: Upload the report

Back in the dashboard, on the host detail page, upload `report.txt`. PatchBase parses the report, creates a snapshot, and matches it against advisory databases — same as agent or SSH pull snapshots.

You should see the host's OS details, package count, and available updates populate immediately.

## When to use manual mode

Manual mode is a good fit when:

- The host is **air-gapped** and can't reach the PatchBase server
- You're doing a **one-time assessment** and don't want to set up ongoing collection
- Security policies prevent both agent installation and server-initiated SSH
- You want to import data from a host before setting up agent or SSH pull mode

The downside is that manual mode doesn't keep the snapshot up to date — you'd need to re-run and re-upload whenever you want fresh data. For ongoing monitoring, switch to [agent mode](./agent-mode) or [SSH pull mode](./ssh-pull-mode) once you can.