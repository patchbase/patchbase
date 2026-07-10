# Onboarding overview

PatchBase supports three ways to get a host's data into the server. You can mix and match across your fleet — each host uses whatever mode fits best.

## Agent mode

Install the PatchBase agent on the host. The agent enrolls with the server using a registration token, then periodically collects a snapshot and sends it back.

**Best for:** hosts where you can install a package and want automatic, scheduled collection without the server needing SSH access.

[Read the agent onboarding guide →](./agent-mode)

## SSH pull mode

No agent needed. The server SSHes into the host on a schedule, runs a collection script, parses the output, and stores the snapshot. PatchBase generates and manages SSH keys for you.

**Best for:** hosts where you can't or don't want to install additional software, but where the server can reach them over SSH.

[Read the SSH pull onboarding guide →](./ssh-pull-mode)

## Manual mode

Download a collection script from the dashboard, run it on the host manually, then upload the resulting report. No agent, no SSH connection from the server.

**Best for:** air-gapped hosts, one-off assessments, or environments where neither agent installation nor server-initiated SSH is possible.

[Read the manual onboarding guide →](./manual-mode)

## Comparison

| Feature | Agent | SSH pull | Manual |
|---------|-------|----------|--------|
| Automatic collection | Yes (systemd timer) | Yes (server-initiated) | No |
| Agent installation required | Yes | No | No |
| Server SSH access required | No | Yes | No |
| Real-time updates | Scheduled | Scheduled | On upload |
| Best for | Standard hosts | Servers without agents | Air-gapped / one-offs |