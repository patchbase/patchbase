# Agent collector design

The agent's collector module (`agent/internal/collector/`) is responsible for gathering all system data that makes up a snapshot.

## Collection pipeline

```
CollectSnapshot()
    │
    ├── ReadOsRelease()       → os-release ID, name, version
    ├── NormalizeOsFamily()   → "apt" or "rpm"
    ├── ParseMajorVersion()   → e.g., "10.2" → 10
    ├── ReadMachineID()      → /etc/machine-id
    ├── os.Hostname()
    ├── getUnameInfo()       → architecture, kernel release
    ├── DetectArchitecture()  → x86_64, aarch64, riscv64
    ├── ReadUptime()          → /proc/uptime → boot time
    ├── CollectUpgradablePackages() → apt list --upgradable / dnf check-update
    ├── RunningKernelNEVRA()  → kernel name-epoch:version-release.arch
    ├── CollectInstalledPackages() → dpkg-query / rpm -qa
    └── CollectEnabledRepos() → apt sources / dnf repolist
```

## Package collection

### APT systems

Uses `dpkg-query` with a pipe-delimited format:

```bash
dpkg-query -W -f='${Package}|${Version}|${Architecture}|${Maintainer}|${source:Package}\n'
```

Each line is parsed into name, epoch, version, release, architecture, vendor (maintainer), and source package. Debian version parsing handles epoch prefixes (`1:`) and upstream/release splitting on `-`.

### RPM systems

Uses `rpm -qa` with a custom query format:

```bash
rpm -qa --queryformat "%{NAME}|%{EPOCHNUM}|%{VERSION}|%{RELEASE}|%{ARCH}|%{SOURCERPM}|%{VENDOR}\n"
```

Each line is parsed into name, epoch, version, release, architecture, source RPM, and vendor.

## Available updates

### APT

```bash
apt list --upgradable
```

The output is parsed line by line, looking for entries containing `/` (repo separator) and `[upgradable from:]`.

### RPM

```bash
dnf -q --cacheonly check-update
# or
yum check-update -q
```

The output is parsed to count available updates, filtering out metadata lines and obsolete notices.

## Repository collection

### APT

Reads `deb` lines from:
- `/etc/apt/sources.list`
- `/etc/apt/sources.list.d/*.list`
- `/etc/apt/sources.list.d/*.sources` (deb822 format)

### RPM

Uses `dnf repolist` or `yum repolist` to list enabled repositories with their IDs and labels.

## ExecRunner abstraction

All shell command execution goes through an `ExecRunner` interface, which makes the collector testable with mock runners. In production, `DefaultExecRunner` uses `os/exec`.