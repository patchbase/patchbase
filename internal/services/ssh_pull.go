package services

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

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

const sshPullDetectOSScript = `
. /etc/os-release 2>/dev/null || true
echo "${ID:-unknown}|${ID_LIKE:-}"
`

const sshPullReportScriptAPT = `
h=$(hostname 2>/dev/null || true)
a=$(uname -m 2>/dev/null || true)
k=$(uname -r 2>/dev/null || true)
m=$(cat /etc/machine-id 2>/dev/null || true)
ip=$(hostname -I 2>/dev/null | awk '{print $1}')
b=$(awk '/^btime / {print $2}' /proc/stat 2>/dev/null || true)
. /etc/os-release 2>/dev/null || true

echo "_PB_METADATA_HOSTNAME=$h"
echo "_PB_METADATA_ARCH=$a"
echo "_PB_METADATA_KERNEL=$k"
echo "_PB_METADATA_MACHINE_ID=$m"
echo "_PB_METADATA_IP=$ip"
echo "_PB_METADATA_BOOT_TIME=$b"
echo "_PB_METADATA_OS_ID=${ID:-unknown}"
echo "_PB_METADATA_OS_ID_LIKE=${ID_LIKE:-}"
echo "_PB_METADATA_OS_NAME=${NAME:-Unknown}"
echo "_PB_METADATA_OS_VERSION=${VERSION_ID:-unknown}"

echo "---UPDATES_START---"
apt list --upgradable 2>/dev/null || true

echo "---PACKAGES_START---"
dpkg-query -W -f='${Package}|${Version}|${Architecture}|${Maintainer}|${source:Package}\n' 2>/dev/null || true

echo "---REPOS_START---"
grep -h -r -d skip "^deb " /etc/apt/sources.list /etc/apt/sources.list.d/ 2>/dev/null || true
awk '/^Suites:[[:space:]]*/ { for (i = 2; i <= NF; i++) print "deb http://deb822.local " $i }' /etc/apt/sources.list.d/*.sources 2>/dev/null || true
`

const sshPullReportScriptRPM = `
h=$(hostname 2>/dev/null || true)
a=$(uname -m 2>/dev/null || true)
k=$(uname -r 2>/dev/null || true)
m=$(cat /etc/machine-id 2>/dev/null || true)
ip=$(hostname -I 2>/dev/null | awk '{print $1}')
b=$(awk '/^btime / {print $2}' /proc/stat 2>/dev/null || true)
. /etc/os-release 2>/dev/null || true

echo "_PB_METADATA_HOSTNAME=$h"
echo "_PB_METADATA_ARCH=$a"
echo "_PB_METADATA_KERNEL=$k"
echo "_PB_METADATA_MACHINE_ID=$m"
echo "_PB_METADATA_IP=$ip"
echo "_PB_METADATA_BOOT_TIME=$b"
echo "_PB_METADATA_OS_ID=${ID:-unknown}"
echo "_PB_METADATA_OS_ID_LIKE=${ID_LIKE:-}"
echo "_PB_METADATA_OS_NAME=${NAME:-Unknown}"
echo "_PB_METADATA_OS_VERSION=${VERSION_ID:-unknown}"

echo "---UPDATES_START---"
if command -v dnf >/dev/null 2>&1; then
	dnf -q --cacheonly check-update 2>/dev/null || true
elif command -v yum >/dev/null 2>&1; then
	yum check-update -q 2>/dev/null || true
fi

echo "---PACKAGES_START---"
rpm -qa --queryformat "%{NAME}|%{EPOCHNUM}|%{VERSION}|%{RELEASE}|%{ARCH}|%{SOURCERPM}|%{VENDOR}\n" 2>/dev/null || true

echo "---REPOS_START---"
if command -v dnf >/dev/null 2>&1; then
	dnf repolist -q 2>/dev/null || true
elif command -v yum >/dev/null 2>&1; then
	yum repolist -q 2>/dev/null || true
fi
`

func (r defaultSSHPullRunner) Collect(ctx context.Context, privateKeyPEM string, user string, host string) (SSHPullResult, error) {
	sshHost, sshPort, err := net.SplitHostPort(host)
	if err != nil {
		sshHost = host
		sshPort = "22"
	}

	detectOutput, err := runSSHScript(ctx, privateKeyPEM, user, sshHost, sshPort, sshPullDetectOSScript)
	if err != nil {
		return SSHPullResult{}, err
	}

	osFamily := detectOSFamilyFromReleaseOutput(detectOutput)
	script, err := collectorScriptForOSFamily(osFamily)
	if err != nil {
		return SSHPullResult{}, fmt.Errorf("detect os family for ssh pull: %w", err)
	}

	output, err := runSSHScript(ctx, privateKeyPEM, user, sshHost, sshPort, script)
	if err != nil {
		return SSHPullResult{}, err
	}

	collectedAt := time.Now().UTC()
	return ParseSSHPullReport(output, collectedAt)
}

func runSSHScript(ctx context.Context, privateKeyPEM string, user string, host string, port string, script string) ([]byte, error) {
	signer, err := ssh.ParsePrivateKey([]byte(privateKeyPEM))
	if err != nil {
		return nil, &SSHPullError{
			ExitCode: -1,
			Message:  fmt.Sprintf("parse private key: %v", err),
			Err:      err,
		}
	}
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         20 * time.Second,
	}

	address := net.JoinHostPort(host, port)

	var d net.Dialer
	d.Timeout = config.Timeout
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, &SSHPullError{
			ExitCode: -1,
			Message:  err.Error(),
			Err:      err,
		}
	}
	if err := conn.SetDeadline(time.Now().Add(config.Timeout)); err != nil {
		conn.Close() // nolint:errcheck
		return nil, &SSHPullError{
			ExitCode: -1,
			Message:  err.Error(),
			Err:      err,
		}
	}

	done := make(chan struct{})
	defer close(done)

	// golang.org/x/crypto/ssh does not support context cancellation directly on Session.
	// This goroutine listens for context cancellation and forces the connection to
	// close, which interrupts any blocking SSH handshake or session.CombinedOutput() call.
	go func() {
		select {
		case <-ctx.Done():
			conn.Close() // nolint:errcheck
		case <-done:
			// Finished normally, let the goroutine exit cleanly
		}
	}()

	c, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		conn.Close() // nolint:errcheck
		return nil, &SSHPullError{
			ExitCode: -1,
			Message:  err.Error(),
			Err:      err,
		}
	}
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close() // nolint:errcheck

	session, err := client.NewSession()
	if err != nil {
		return nil, &SSHPullError{
			ExitCode: -1,
			Message:  err.Error(),
			Err:      err,
		}
	}
	defer session.Close() // nolint:errcheck
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, &SSHPullError{
			ExitCode: -1,
			Message:  err.Error(),
			Err:      err,
		}
	}

	cmdStr := "sh -lc '" + strings.ReplaceAll(script, "'", "'\"'\"'") + "'"
	output, err := session.CombinedOutput(cmdStr)
	if err == nil {
		return output, nil
	}

	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}
	exitCode := -1
	if exitErr, ok := err.(*ssh.ExitError); ok {
		exitCode = exitErr.ExitStatus()
	}
	return nil, &SSHPullError{
		ExitCode: exitCode,
		Message:  message,
		Err:      err,
	}
}

func collectorScriptForOSFamily(osFamily string) (string, error) {
	switch normalizeOSFamilyString(osFamily) {
	case "apt":
		return sshPullReportScriptAPT, nil
	case "rpm":
		return sshPullReportScriptRPM, nil
	default:
		return "", fmt.Errorf("unsupported os family %q", osFamily)
	}
}

func detectOSFamilyFromReleaseOutput(output []byte) string {
	line := strings.TrimSpace(string(output))
	if line == "" {
		return ""
	}

	for _, candidate := range strings.Split(line, "\n") {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}

		id, idLike, hasLike := strings.Cut(trimmed, "|")
		if family := normalizeOSFamilyString(id); family != "" {
			return family
		}
		if hasLike {
			for _, token := range strings.Fields(strings.ReplaceAll(idLike, ",", " ")) {
				if family := normalizeOSFamilyString(token); family != "" {
					return family
				}
			}
		}
	}

	return ""
}

func normalizeOSFamilyString(raw string) string {
	switch strings.ToLower(strings.TrimSpace(CleanQuote(raw))) {
	case "apt", "debian", "ubuntu", "linuxmint":
		return "apt"
	case "rpm", "rocky", "rhel", "almalinux", "centos", "fedora":
		return "rpm"
	default:
		return ""
	}
}

func ParseSSHPullReport(output []byte, collectedAt time.Time) (SSHPullResult, error) {
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

	osFamily := normalizeOSFamilyString(fields["OS_ID"])
	if osFamily == "" {
		for _, token := range strings.Fields(strings.ReplaceAll(fields["OS_ID_LIKE"], ",", " ")) {
			osFamily = normalizeOSFamilyString(token)
			if osFamily != "" {
				break
			}
		}
	}
	if osFamily == "" {
		osFamily = "unknown"
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
	upgradablePackages := ParseUpgradablePackages(osFamily, updatesSection)

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
		Packages:           packages,
		Repos:              repos,
		UpgradablePackages: upgradablePackages,
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

func ParseUpgradablePackages(osFamily string, output string) []*agentpb.Package {
	items := make([]*agentpb.Package, 0)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var pkg *agentpb.Package
		switch osFamily {
		case "apt":
			pkg = parseAptUpgradableLine(line)
		case "rpm":
			pkg = parseRpmUpgradableLine(line)
		}
		if pkg != nil {
			items = append(items, pkg)
		}
	}

	return items
}

func parseAptUpgradableLine(line string) *agentpb.Package {
	if strings.HasPrefix(line, "Listing...") || strings.HasPrefix(line, "WARNING:") || strings.HasPrefix(line, "N:") {
		return nil
	}
	if !strings.Contains(line, "[upgradable from:") {
		return nil
	}

	fields := strings.Fields(line)
	if len(fields) < 3 {
		return nil
	}

	nameField := fields[0]
	name, repoOrigin, ok := strings.Cut(nameField, "/")
	if !ok || strings.TrimSpace(name) == "" {
		return nil
	}

	version := strings.TrimSpace(fields[1])
	arch := strings.TrimSpace(fields[2])
	nevra := fmt.Sprintf("%s-%s", name, version)
	if arch != "" {
		nevra = fmt.Sprintf("%s.%s", nevra, arch)
	}

	return &agentpb.Package{
		Name:       name,
		Version:    version,
		Arch:       arch,
		RepoOrigin: repoOrigin,
		Nevra:      nevra,
	}
}

func parseRpmUpgradableLine(line string) *agentpb.Package {
	if strings.HasPrefix(line, "Last metadata expiration check:") {
		return nil
	}
	if line == "Obsoleting Packages" || line == "Obsoleted Packages" {
		return nil
	}

	fields := strings.Fields(line)
	if len(fields) < 3 {
		return nil
	}

	nameArch := fields[0]
	targetVersion := fields[1]
	repoOrigin := fields[2]

	name, arch, ok := strings.Cut(nameArch, ".")
	if !ok || strings.TrimSpace(name) == "" {
		return nil
	}

	epoch, versionRelease := int32(0), targetVersion
	if epochPart, rest, cut := strings.Cut(targetVersion, ":"); cut {
		if parsed, err := strconv.ParseInt(epochPart, 10, 32); err == nil {
			epoch = int32(parsed)
			versionRelease = rest
		}
	}

	version := versionRelease
	release := ""
	if idx := strings.LastIndex(versionRelease, "-"); idx > 0 && idx+1 < len(versionRelease) {
		version = versionRelease[:idx]
		release = versionRelease[idx+1:]
	}

	nevra := fmt.Sprintf("%s-%d:%s", name, epoch, version)
	if release != "" {
		nevra = fmt.Sprintf("%s-%s", nevra, release)
	}
	if arch != "" {
		nevra = fmt.Sprintf("%s.%s", nevra, arch)
	}

	return &agentpb.Package{
		Name:       name,
		Epoch:      epoch,
		Version:    version,
		Release:    release,
		Arch:       arch,
		RepoOrigin: repoOrigin,
		Nevra:      nevra,
	}
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
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(fields))
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
	sourcePackage := optionalStr(fields[4])

	nevra := formatPackageIdentifier(name, epoch, version, release, arch)
	return &agentpb.Package{
		Name:      name,
		Epoch:     epoch,
		Version:   version,
		Release:   release,
		Arch:      arch,
		Vendor:    vendor,
		Nevra:     nevra,
		SourceRpm: sourcePackage,
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
