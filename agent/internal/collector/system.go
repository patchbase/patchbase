package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/afero"
	agent "go.patchbase.net/proto/agent"
)

type ExecRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type OsRelease struct {
	ID        string
	Name      string
	VersionID string
}

func ReadOsRelease(fs afero.Fs) (OsRelease, error) {
	data, err := afero.ReadFile(fs, "/etc/os-release")
	if err != nil {
		return OsRelease{}, fmt.Errorf("read /etc/os-release: %w", err)
	}
	return ParseOsRelease(string(data))
}

func ParseOsRelease(contents string) (OsRelease, error) {
	var id, name, versionID string

	scanner := bufio.NewScanner(strings.NewReader(contents))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := stripQuotes(strings.TrimSpace(line[idx+1:]))

		switch key {
		case "ID":
			id = value
		case "NAME":
			name = value
		case "VERSION_ID":
			versionID = value
		}
	}

	if id == "" {
		return OsRelease{}, fmt.Errorf("missing ID in /etc/os-release")
	}
	if name == "" {
		return OsRelease{}, fmt.Errorf("missing NAME in /etc/os-release")
	}
	if versionID == "" {
		return OsRelease{}, fmt.Errorf("missing VERSION_ID in /etc/os-release")
	}

	return OsRelease{ID: id, Name: name, VersionID: versionID}, nil
}

func NormalizeOsFamily(distroID string) (agent.OsFamily, error) {
	switch strings.ToLower(distroID) {
	case "rhel", "rocky", "almalinux", "centos", "ol":
		return agent.OsFamily_OS_FAMILY_RPM, nil
	case "debian", "ubuntu", "linuxmint", "pop", "raspbian":
		return agent.OsFamily_OS_FAMILY_APT, nil
	default:
		return agent.OsFamily_OS_FAMILY_UNSPECIFIED, fmt.Errorf("unsupported os family: %s", distroID)
	}
}

func ParseMajorVersion(versionID string) (int32, error) {
	idx := strings.Index(versionID, ".")
	var majorStr string
	if idx >= 0 {
		majorStr = versionID[:idx]
	} else {
		majorStr = versionID
	}

	var major int32
	_, err := fmt.Sscanf(majorStr, "%d", &major)
	if err != nil {
		return 0, fmt.Errorf("parse major version from %q: %w", versionID, err)
	}
	return major, nil
}

func ReadMachineID(fs afero.Fs) (string, error) {
	candidates := []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}
	for _, path := range candidates {
		data, err := afero.ReadFile(fs, path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("read %s: %w", path, err)
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed != "" {
			return trimmed, nil
		}
	}
	return "", fmt.Errorf("machine-id not found")
}

func DetectArchitecture(unameMachine string) (agent.Architecture, error) {
	switch unameMachine {
	case "x86_64":
		return agent.Architecture_ARCHITECTURE_X86_64, nil
	case "aarch64":
		return agent.Architecture_ARCHITECTURE_AARCH64, nil
	case "riscv64":
		return agent.Architecture_ARCHITECTURE_RISCV64, nil
	default:
		return agent.Architecture_ARCHITECTURE_UNSPECIFIED, fmt.Errorf("unsupported architecture: %s", unameMachine)
	}
}

func RunningKernelNEVRA(ctx context.Context, runner ExecRunner, osFamily agent.OsFamily, unameRelease string) (string, error) {
	switch osFamily {
	case agent.OsFamily_OS_FAMILY_RPM:
		return runningRPMKernelNEVRA(ctx, runner, unameRelease)
	case agent.OsFamily_OS_FAMILY_APT:
		return unameRelease, nil
	default:
		return "", fmt.Errorf("unsupported os family: %s", osFamily.String())
	}
}

func runningRPMKernelNEVRA(ctx context.Context, runner ExecRunner, unameRelease string) (string, error) {
	query := fmt.Sprintf("kernel-uname-r = %s", unameRelease)
	output, err := runner.Run(ctx,
		"rpm", "-q", "--whatprovides", query,
		"--queryformat", "%{EPOCHNUM}:%{VERSION}-%{RELEASE}.%{ARCH}\n",
	)
	if err != nil {
		return unameRelease, nil
	}

	line := strings.TrimSpace(string(output))
	if line == "" {
		return unameRelease, nil
	}

	if idx := strings.Index(line, "\n"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}

	if strings.HasPrefix(line, "0:") {
		line = strings.TrimPrefix(line, "0:")
	}

	return line, nil
}

func ReadUptime(fs afero.Fs) (int64, error) {
	data, err := afero.ReadFile(fs, "/proc/uptime")
	if err != nil {
		return 0, fmt.Errorf("read /proc/uptime: %w", err)
	}

	contents := strings.TrimSpace(string(data))
	parts := strings.SplitN(contents, " ", 2)
	if len(parts) < 1 {
		return 0, fmt.Errorf("invalid /proc/uptime format")
	}

	var uptimeFloat float64
	_, err = fmt.Sscanf(parts[0], "%f", &uptimeFloat)
	if err != nil {
		return 0, fmt.Errorf("parse uptime from %q: %w", parts[0], err)
	}

	return int64(uptimeFloat), nil
}

func stripQuotes(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

type realExecRunner struct{}

func (realExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.Stderr, err
		}
		return nil, err
	}
	return output, nil
}

var DefaultExecRunner ExecRunner = realExecRunner{}
