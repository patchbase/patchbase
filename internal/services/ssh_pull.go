package services

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/samber/do/v2"
	agentpb "go.patchbase.net/proto/agent"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type SSHPullResult struct {
	MachineID        string
	Hostname         string
	IPAddress        string
	OSFamily         string
	OSName           string
	OSVersion        string
	OSMajor          int32
	Architecture     string
	RunningKernel    string
	CollectedAt      time.Time
	BootTime         *time.Time
	AvailableUpdates int32
	HasProcessData   bool
	Payload          []byte
	OverallAction    string
	CriticalCount    int32
	ImportantCount   int32
	ModerateCount    int32
	ActionableCount  int32
	NeedsReboot      int32
	NeedsRestart     int32
	NoFix            int32
	Unknown          int32
}

type SSHPullRunner interface {
	Collect(ctx context.Context, privateKeyPEM string, user string, host string) (SSHPullResult, error)
	TryConnect(ctx context.Context, address string) (string, string)
}

type SSHPullError struct {
	ExitCode int
	Message  string
	Err      error
}

func (e *SSHPullError) Error() string {
	return fmt.Sprintf("ssh collection failed (exit status %d): %s", e.ExitCode, e.Message)
}

func (e *SSHPullError) Unwrap() error {
	return e.Err
}

func NewSSHPullRunner(i do.Injector) (SSHPullRunner, error) {
	return defaultSSHPullRunner{}, nil
}

type defaultSSHPullRunner struct{}

func (r defaultSSHPullRunner) Collect(ctx context.Context, privateKeyPEM string, user string, host string) (SSHPullResult, error) {
	tmpFile, err := os.CreateTemp("", "patchbase-ssh-key-*")
	if err != nil {
		return SSHPullResult{}, fmt.Errorf("create temporary private key file: %w", err)
	}
	defer os.Remove(tmpFile.Name()) // nolint:errcheck

	if _, err := tmpFile.WriteString(privateKeyPEM); err != nil {
		tmpFile.Close() // nolint:errcheck
		return SSHPullResult{}, fmt.Errorf("write temporary private key file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return SSHPullResult{}, fmt.Errorf("close temporary private key file: %w", err)
	}
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		return SSHPullResult{}, fmt.Errorf("chmod temporary private key file: %w", err)
	}

	sshHost, sshPort, err := net.SplitHostPort(host)
	if err != nil {
		sshHost = host
		sshPort = "22"
	}

	cmd := exec.CommandContext(
		ctx,
		"ssh",
		"-p", sshPort,
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=20",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-i", tmpFile.Name(),
		fmt.Sprintf("%s@%s", user, sshHost),
		"sh", "-lc",
		`h=$(hostname 2>/dev/null || true); a=$(uname -m 2>/dev/null || true); k=$(uname -r 2>/dev/null || true); m=$(cat /etc/machine-id 2>/dev/null || true); ip=$(hostname -I 2>/dev/null | awk '{print $1}'); b=$(awk '/^btime / {print $2}' /proc/stat 2>/dev/null || true); . /etc/os-release 2>/dev/null || true; echo "_PB_METADATA_HOSTNAME=$h"; echo "_PB_METADATA_ARCH=$a"; echo "_PB_METADATA_KERNEL=$k"; echo "_PB_METADATA_MACHINE_ID=$m"; echo "_PB_METADATA_IP=$ip"; echo "_PB_METADATA_BOOT_TIME=$b"; echo "_PB_METADATA_OS_ID=${ID:-unknown}"; echo "_PB_METADATA_OS_NAME=${NAME:-Unknown}"; echo "_PB_METADATA_OS_VERSION=${VERSION_ID:-unknown}"; echo "---UPDATES_START---"; if command -v apt >/dev/null 2>&1; then apt list --upgradable 2>/dev/null; elif command -v dnf >/dev/null 2>&1; then dnf -q --cacheonly check-update 2>/dev/null || true; elif command -v yum >/dev/null 2>&1; then yum check-update -q 2>/dev/null || true; fi; echo "---PACKAGES_START---"; if command -v rpm >/dev/null 2>&1; then rpm -qa --queryformat "%{NAME}|%{EPOCHNUM}|%{VERSION}|%{RELEASE}|%{ARCH}|%{SOURCERPM}|%{VENDOR}\n" 2>/dev/null; elif command -v dpkg-query >/dev/null 2>&1; then dpkg-query -W -f='${Package}|${Version}|${Architecture}|${Maintainer}\n' 2>/dev/null; fi; echo "---REPOS_START---"; if command -v dnf >/dev/null 2>&1; then dnf repolist -q 2>/dev/null; elif command -v yum >/dev/null 2>&1; then yum repolist -q 2>/dev/null; elif [ -d /etc/apt ]; then grep -h -r -d skip "^deb " /etc/apt/sources.list /etc/apt/sources.list.d/ 2>/dev/null; fi`,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		exitCode := -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		return SSHPullResult{}, &SSHPullError{
			ExitCode: exitCode,
			Message:  message,
			Err:      err,
		}
	}

	outputStr := string(output)
	parts := strings.Split(outputStr, "---UPDATES_START---\n")
	if len(parts) < 2 {
		parts = strings.Split(outputStr, "---UPDATES_START---")
	}

	fields := map[string]string{}
	firstPartLines := strings.Split(parts[0], "\n")
	for _, line := range firstPartLines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "_PB_METADATA_") {
			continue
		}
		trimmedLine := strings.TrimPrefix(line, "_PB_METADATA_")
		key, value, ok := strings.Cut(trimmedLine, "=")
		if !ok {
			continue
		}
		fields[key] = CleanQuote(value)
	}

	bootPtr := (*time.Time)(nil)
	if rawBoot := strings.TrimSpace(fields["BOOT_TIME"]); rawBoot != "" {
		if unixSeconds, err := strconv.ParseInt(rawBoot, 10, 64); err == nil && unixSeconds > 0 {
			boot := time.Unix(unixSeconds, 0).UTC()
			bootPtr = &boot
		}
	}

	osID := strings.ToLower(strings.TrimSpace(fields["OS_ID"]))
	osFamily := "unknown"
	switch osID {
	case "ubuntu", "debian":
		osFamily = "apt"
	case "rocky", "rhel", "almalinux", "centos", "fedora":
		osFamily = "rpm"
	}

	osMajor := int32(0)
	if rawVersion := strings.TrimSpace(fields["OS_VERSION"]); rawVersion != "" {
		majorPart := strings.Split(rawVersion, ".")[0]
		if parsed, err := strconv.ParseInt(majorPart, 10, 32); err == nil {
			osMajor = int32(parsed)
		}
	}

	arch := strings.TrimSpace(fields["ARCH"])
	if arch == "" {
		arch = "unknown"
	}

	collectedAt := time.Now().UTC()

	var packages []*agentpb.Package
	var repos []*agentpb.Repo
	updatesSection := ""
	packagesSection := ""
	reposSection := ""

	if len(parts) >= 2 {
		updatesAndRest := parts[1]
		subParts := strings.Split(updatesAndRest, "---PACKAGES_START---")
		updatesSection = subParts[0]

		if len(subParts) >= 2 {
			packagesAndRepos := subParts[1]
			pkgParts := strings.Split(packagesAndRepos, "---REPOS_START---")
			packagesSection = pkgParts[0]

			if len(pkgParts) >= 2 {
				reposSection = pkgParts[1]
			}
		}
	}

	// Parse packages
	pkgScanner := bufio.NewScanner(strings.NewReader(packagesSection))
	for pkgScanner.Scan() {
		line := strings.TrimSpace(pkgScanner.Text())
		if line == "" {
			continue
		}
		var pkg *agentpb.Package
		var err error
		switch osFamily {
		case "rpm":
			pkg, err = parseRPMPackageLine(line)
		case "apt":
			pkg, err = parseAPTPackageLine(line)
		}
		if err == nil && pkg != nil {
			packages = append(packages, pkg)
		}
	}

	// Parse repos
	repoScanner := bufio.NewScanner(strings.NewReader(reposSection))
	lineNo := 0
	for repoScanner.Scan() {
		lineNo++
		line := strings.TrimSpace(repoScanner.Text())
		if line == "" {
			continue
		}
		var repo *agentpb.Repo
		switch osFamily {
		case "rpm":
			repo = parseRPMRepoLine(line)
		case "apt":
			repo = parseAPTRepoLine(line, lineNo)
		}
		if repo != nil {
			repos = append(repos, repo)
		}
	}

	var availableUpdates int32
	if osFamily != "unknown" {
		switch osFamily {
		case "apt":
			availableUpdates = CountAptPackageUpdates(updatesSection)
		case "rpm":
			availableUpdates = CountRpmPackageUpdates(updatesSection)
		}
	}

	var pbArch agentpb.Architecture
	switch arch {
	case "x86_64", "amd64":
		pbArch = agentpb.Architecture_ARCHITECTURE_X86_64
	case "aarch64", "arm64":
		pbArch = agentpb.Architecture_ARCHITECTURE_AARCH64
	case "riscv64":
		pbArch = agentpb.Architecture_ARCHITECTURE_RISCV64
	default:
		pbArch = agentpb.Architecture_ARCHITECTURE_UNSPECIFIED
	}

	var pbOsFamily agentpb.OsFamily
	switch osFamily {
	case "rpm":
		pbOsFamily = agentpb.OsFamily_OS_FAMILY_RPM
	case "apt":
		pbOsFamily = agentpb.OsFamily_OS_FAMILY_APT
	default:
		pbOsFamily = agentpb.OsFamily_OS_FAMILY_UNSPECIFIED
	}

	agentSnap := &agentpb.AgentSnapshot{
		SchemaVersion: "1.0",
		SentAt:        timestamppb.New(collectedAt),
		Host: &agentpb.Host{
			Hostname:                    fallback(fields["HOSTNAME"], "unknown"),
			MachineId:                   fallback(fields["MACHINE_ID"], "unknown"),
			IpAddresses:                 []string{fallback(fields["IP"], "unknown")},
			OsFamily:                    pbOsFamily,
			OsName:                      fallback(fields["OS_NAME"], "Unknown"),
			OsVersion:                   fallback(fields["OS_VERSION"], "unknown"),
			OsMajor:                     osMajor,
			Architecture:                pbArch,
			AvailablePackageUpdateCount: availableUpdates,
		},
		Packages: packages,
		Repos:    repos,
		Runtime: &agentpb.Runtime{
			KernelRunning: fallback(fields["KERNEL"], "unknown"),
		},
	}
	if bootPtr != nil {
		agentSnap.Host.BootTime = timestamppb.New(*bootPtr)
	}

	payload, err := proto.Marshal(agentSnap)
	if err != nil {
		return SSHPullResult{}, fmt.Errorf("marshal agent snapshot proto: %w", err)
	}

	overallAction := "none"
	if availableUpdates > 0 {
		overallAction = "update_package"
	}

	return SSHPullResult{
		MachineID:        strings.TrimSpace(fields["MACHINE_ID"]),
		Hostname:         strings.TrimSpace(fields["HOSTNAME"]),
		IPAddress:        strings.TrimSpace(fields["IP"]),
		OSFamily:         osFamily,
		OSName:           fallback(fields["OS_NAME"], "Unknown"),
		OSVersion:        fallback(fields["OS_VERSION"], "unknown"),
		OSMajor:          osMajor,
		Architecture:     arch,
		RunningKernel:    strings.TrimSpace(fields["KERNEL"]),
		CollectedAt:      collectedAt,
		BootTime:         bootPtr,
		AvailableUpdates: availableUpdates,
		HasProcessData:   false,
		Payload:          payload,
		OverallAction:    overallAction,
		CriticalCount:    0,
		ImportantCount:   0,
		ModerateCount:    0,
		ActionableCount:  availableUpdates,
		NeedsReboot:      0,
		NeedsRestart:     0,
		NoFix:            0,
		Unknown:          0,
	}, nil
}

func CleanQuote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

func CountAptPackageUpdates(output string) int32 {
	var count int32
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Listing...") || strings.HasPrefix(line, "WARNING:") || strings.HasPrefix(line, "N:") {
			continue
		}
		if !strings.Contains(line, "/") {
			continue
		}
		if !strings.Contains(line, "[upgradable from:") {
			continue
		}
		count++
	}
	return count
}

func CountRpmPackageUpdates(output string) int32 {
	var count int32
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Last metadata expiration check:") {
			continue
		}
		if line == "Obsoleting Packages" || line == "Obsoleted Packages" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		first := fields[0]
		second := fields[1]

		if !strings.Contains(first, ".") {
			continue
		}
		if !strings.ContainsAny(second, "0123456789") {
			continue
		}

		count++
	}
	return count
}

func fallback(value string, defaultValue string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultValue
	}
	return trimmed
}

func (r defaultSSHPullRunner) TryConnect(ctx context.Context, address string) (string, string) {
	conn, dialErr := net.DialTimeout("tcp", address, 5*time.Second)
	if dialErr != nil {
		return "failed", dialErr.Error()
	}
	conn.Close() // nolint:errcheck
	return "success", ""
}

func parseRPMPackageLine(line string) (*agentpb.Package, error) {
	fields := strings.Split(line, "|")
	if len(fields) != 7 {
		return nil, fmt.Errorf("expected 7 fields, got %d", len(fields))
	}
	epoch, err := parseEpoch(fields[1])
	if err != nil {
		return nil, fmt.Errorf("parse epoch: %w", err)
	}
	name := fields[0]
	version := fields[2]
	release := fields[3]
	arch := fields[4]
	nevra := fmt.Sprintf("%s-%d:%s-%s.%s", name, epoch, version, release, arch)

	return &agentpb.Package{
		Name:      name,
		Epoch:     epoch,
		Version:   version,
		Release:   release,
		Arch:      arch,
		Nevra:     nevra,
		SourceRpm: optionalStr(fields[5]),
		Vendor:    optionalStr(fields[6]),
	}, nil
}

func parseAPTPackageLine(line string) (*agentpb.Package, error) {
	fields := strings.Split(line, "|")
	if len(fields) != 4 {
		return nil, fmt.Errorf("expected 4 fields, got %d", len(fields))
	}

	name := strings.TrimSpace(fields[0])
	if name == "" {
		return nil, fmt.Errorf("empty package name")
	}

	epoch, version, release, err := parseDebianVersion(strings.TrimSpace(fields[1]))
	if err != nil {
		return nil, fmt.Errorf("parse debian version: %w", err)
	}

	arch := strings.TrimSpace(fields[2])
	vendor := optionalStr(fields[3])

	nevra := formatPackageIdentifier(name, epoch, version, release, arch)
	return &agentpb.Package{
		Name:    name,
		Epoch:   epoch,
		Version: version,
		Release: release,
		Arch:    arch,
		Vendor:  vendor,
		Nevra:   nevra,
	}, nil
}

func formatPackageIdentifier(name string, epoch int32, version, release, arch string) string {
	if release == "" && arch == "" {
		return fmt.Sprintf("%s-%d:%s", name, epoch, version)
	}
	if release == "" {
		return fmt.Sprintf("%s-%d:%s.%s", name, epoch, version, arch)
	}
	if arch == "" {
		return fmt.Sprintf("%s-%d:%s-%s", name, epoch, version, release)
	}
	return fmt.Sprintf("%s-%d:%s-%s.%s", name, epoch, version, release, arch)
}

func parseDebianVersion(value string) (int32, string, string, error) {
	version := strings.TrimSpace(value)
	if version == "" {
		return 0, "", "", fmt.Errorf("empty version")
	}

	epoch := int32(0)
	if idx := strings.Index(version, ":"); idx >= 0 {
		parsed, err := parseEpoch(version[:idx])
		if err != nil {
			return 0, "", "", fmt.Errorf("parse epoch: %w", err)
		}
		epoch = parsed
		version = version[idx+1:]
	}

	if version == "" {
		return 0, "", "", fmt.Errorf("missing version after epoch")
	}

	upstream := version
	release := ""
	if idx := strings.LastIndex(version, "-"); idx >= 0 {
		upstream = version[:idx]
		release = version[idx+1:]
	}

	if upstream == "" {
		return 0, "", "", fmt.Errorf("missing upstream version")
	}

	return epoch, upstream, release, nil
}

func parseEpoch(value string) (int32, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "(none)" {
		return 0, nil
	}
	n, err := strconv.ParseInt(trimmed, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse epoch %q: %w", trimmed, err)
	}
	return int32(n), nil
}

func optionalStr(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "(none)" {
		return ""
	}
	return trimmed
}

func parseRPMRepoLine(line string) *agentpb.Repo {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if strings.HasPrefix(line, "repo id") || strings.HasPrefix(line, "Last metadata") {
		return nil
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	repoID := fields[0]
	repoLabel := strings.TrimSpace(strings.TrimPrefix(line, repoID))
	return &agentpb.Repo{
		RepoId:    repoID,
		Enabled:   true,
		RepoLabel: repoLabel,
	}
}

func parseAPTRepoLine(line string, lineNo int) *agentpb.Repo {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "deb ") {
		return nil
	}

	remainder := strings.TrimSpace(strings.TrimPrefix(trimmed, "deb "))
	if strings.HasPrefix(remainder, "[") {
		closing := strings.Index(remainder, "]")
		if closing >= 0 && closing+1 < len(remainder) {
			remainder = strings.TrimSpace(remainder[closing+1:])
		}
	}

	fields := strings.Fields(remainder)
	if len(fields) < 2 {
		return nil
	}
	uri := fields[0]
	suite := fields[1]
	components := fields[2:]

	repoID := "ssh_pull:" + strconv.Itoa(lineNo)
	repoLabel := suite
	if len(components) > 0 {
		repoLabel = suite + " " + strings.Join(components, " ")
	}
	return &agentpb.Repo{
		RepoId:    repoID,
		Enabled:   true,
		RepoLabel: repoLabel,
		Baseurl:   uri,
	}
}
