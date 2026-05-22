package services

import (
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
		`h=$(hostname 2>/dev/null || true); a=$(uname -m 2>/dev/null || true); k=$(uname -r 2>/dev/null || true); m=$(cat /etc/machine-id 2>/dev/null || true); ip=$(hostname -I 2>/dev/null | awk '{print $1}'); b=$(awk '/^btime / {print $2}' /proc/stat 2>/dev/null || true); . /etc/os-release 2>/dev/null || true; echo "HOSTNAME=$h"; echo "ARCH=$a"; echo "KERNEL=$k"; echo "MACHINE_ID=$m"; echo "IP=$ip"; echo "BOOT_TIME=$b"; echo "OS_ID=${ID:-unknown}"; echo "OS_NAME=${NAME:-Unknown}"; echo "OS_VERSION=${VERSION_ID:-unknown}"`,
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

	fields := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		fields[key] = strings.TrimSpace(value)
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
		AvailableUpdates: 0,
		HasProcessData:   false,
		Payload:          payload,
		OverallAction:    "none",
		CriticalCount:    0,
		ImportantCount:   0,
		ModerateCount:    0,
		ActionableCount:  0,
		NeedsReboot:      0,
		NeedsRestart:     0,
		NoFix:            0,
		Unknown:          0,
	}, nil
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
