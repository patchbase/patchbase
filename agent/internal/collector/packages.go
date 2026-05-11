package collector

import (
	"bufio"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	agent "go.patchbase.net/proto/agent"
)

func CollectInstalledPackages(ctx context.Context, runner ExecRunner) ([]*agent.Package, error) {
	output, err := runner.Run(ctx,
		"rpm", "-qa",
		"--queryformat", "%{NAME}|%{EPOCHNUM}|%{VERSION}|%{RELEASE}|%{ARCH}|%{SOURCERPM}|%{VENDOR}\n",
	)
	if err != nil {
		return nil, fmt.Errorf("rpm -qa: %w", err)
	}

	var packages []*agent.Package
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		pkg, err := parsePackageLine(line)
		if err != nil {
			continue
		}
		packages = append(packages, pkg)
	}

	sort.Slice(packages, func(i, j int) bool {
		switch {
		case packages[i].Name != packages[j].Name:
			return packages[i].Name < packages[j].Name
		case packages[i].Arch != packages[j].Arch:
			return packages[i].Arch < packages[j].Arch
		case packages[i].Version != packages[j].Version:
			return packages[i].Version < packages[j].Version
		case packages[i].Release != packages[j].Release:
			return packages[i].Release < packages[j].Release
		case packages[i].Nevra != packages[j].Nevra:
			return packages[i].Nevra < packages[j].Nevra
		default:
			return packages[i].Epoch < packages[j].Epoch
		}
	})

	return packages, nil
}

func parsePackageLine(line string) (*agent.Package, error) {
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

	return &agent.Package{
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

func CollectAvailablePackageUpdateCount(ctx context.Context, runner ExecRunner) (int32, error) {
	output, err := runner.Run(ctx, "dnf", "-q", "--cacheonly", "check-update")
	if err != nil {
		return 0, nil
	}

	return countPackageUpdates(string(output)), nil
}

func countPackageUpdates(output string) int32 {
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
