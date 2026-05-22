package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
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
		`h=$(hostname 2>/dev/null || true); a=$(uname -m 2>/dev/null || true); k=$(uname -r 2>/dev/null || true); m=$(cat /etc/machine-id 2>/dev/null || true); ip=$(hostname -I 2>/dev/null | awk '{print $1}'); b=$(awk '/^btime / {print $2}' /proc/stat 2>/dev/null || true); . /etc/os-release 2>/dev/null || true; echo "_PB_METADATA_HOSTNAME=$h"; echo "_PB_METADATA_ARCH=$a"; echo "_PB_METADATA_KERNEL=$k"; echo "_PB_METADATA_MACHINE_ID=$m"; echo "_PB_METADATA_IP=$ip"; echo "_PB_METADATA_BOOT_TIME=$b"; echo "_PB_METADATA_OS_ID=${ID:-unknown}"; echo "_PB_METADATA_OS_NAME=${NAME:-Unknown}"; echo "_PB_METADATA_OS_VERSION=${VERSION_ID:-unknown}"; echo "---UPDATES_START---"; if command -v apt >/dev/null 2>&1; then apt list --upgradable 2>/dev/null; elif command -v dnf >/dev/null 2>&1; then dnf -q --cacheonly check-update 2>/dev/null || true; elif command -v yum >/dev/null 2>&1; then yum check-update -q 2>/dev/null || true; fi`,
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
		fields[key] = cleanQuote(value)
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
	payload, _ := json.Marshal(map[string]any{
		"source":       "ssh_pull",
		"hostname":     fields["HOSTNAME"],
		"machine_id":   fields["MACHINE_ID"],
		"os_id":        fields["OS_ID"],
		"os_name":      fields["OS_NAME"],
		"os_version":   fields["OS_VERSION"],
		"arch":         fields["ARCH"],
		"kernel":       fields["KERNEL"],
		"ip":           fields["IP"],
		"collected_at": collectedAt,
	})

	var availableUpdates int32
	if len(parts) >= 2 && osFamily != "unknown" {
		updatesSection := parts[1]
		switch osFamily {
		case "apt":
			availableUpdates = countAptPackageUpdates(updatesSection)
		case "rpm":
			availableUpdates = countRpmPackageUpdates(updatesSection)
		}
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

func cleanQuote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

func countAptPackageUpdates(output string) int32 {
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

func countRpmPackageUpdates(output string) int32 {
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
