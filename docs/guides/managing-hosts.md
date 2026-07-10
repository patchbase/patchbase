# Managing hosts

Once you've onboarded hosts, the PatchBase dashboard gives you a single view of your fleet. Here's how to use it.

## Dashboard overview

The main dashboard shows:

- **Total hosts** — how many hosts are registered
- **Need attention** — hosts with available updates or pending advisories
- **Reboot queue** — hosts where a kernel update has been installed but the system hasn't rebooted
- **Unknown / investigate** — hosts where advisory matching couldn't determine a clear status
- **Total advisories** — how many security advisories are currently matched across all hosts
- **Recent advisories** — the latest advisories published by upstream distributions

## Host list

The hosts page shows every registered host with:

- Display name and hostname
- OS family, name, version, and architecture
- Status (approved, pending)
- Vulnerability counts by severity (critical, important, moderate)
- Available updates count
- Whether a reboot or restart is needed
- Last seen timestamp

Click any host to see its detail page.

## Host detail page

Each host has a detail page showing:

- **System info** — OS, kernel, architecture, last boot time, agent version
- **Vulnerable packages** — packages matched against advisories, with the advisory ID and severity
- **Upgradable packages** — packages with newer versions available
- **Kernel posture** — whether the running kernel matches the latest installed kernel
- **Pull job history** (SSH pull mode only) — recent collection runs and their status
- **Latest snapshot** — raw snapshot data

## Approving pending hosts

Hosts that register via the agent appear in **Pending** status. Go to **Hosts → Pending** to review and approve them. Until approved, a host can't send snapshots.

This prevents unwanted hosts from registering with a stolen registration token.

## Deleting a host

From the host detail page, you can delete a host. This removes all its data — snapshots, pull jobs, and access tokens. The action is irreversible.

For SSH pull hosts, deleting also removes the periodic pull job from the scheduler.

## Registration tokens

Registration tokens are used by agents to enroll. You can create multiple tokens (e.g., one per team or environment) and revoke them when no longer needed. A token that's been revoked can't be used for new registrations, but already-enrolled hosts continue to work.

Create tokens from **Hosts → Register → Tokens**.