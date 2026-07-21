// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
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

func CollectInstalledPackages(ctx context.Context, runner ExecRunner, osFamily agent.OsFamily) ([]*agent.Package, error) {
	switch osFamily {
	case agent.OsFamily_OS_FAMILY_RPM:
		return collectInstalledRPMPackages(ctx, runner)
	case agent.OsFamily_OS_FAMILY_APT:
		return collectInstalledAPTPackages(ctx, runner)
	default:
		return nil, fmt.Errorf("unsupported os family: %s", osFamily.String())
	}
}

func collectInstalledRPMPackages(ctx context.Context, runner ExecRunner) ([]*agent.Package, error) {
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

func collectInstalledAPTPackages(ctx context.Context, runner ExecRunner) ([]*agent.Package, error) {
	output, err := runner.Run(ctx,
		"dpkg-query", "-W",
		"-f=${Package}|${Version}|${Architecture}|${Maintainer}|${source:Package}\n",
	)
	if err != nil {
		return nil, fmt.Errorf("dpkg-query -W: %w", err)
	}

	var packages []*agent.Package
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		pkg, err := parseAptPackageLine(line)
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

func parseAptPackageLine(line string) (*agent.Package, error) {
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
	return &agent.Package{
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

func CollectAvailablePackageUpdateCount(ctx context.Context, runner ExecRunner, osFamily agent.OsFamily) (int32, error) {
	switch osFamily {
	case agent.OsFamily_OS_FAMILY_RPM:
		return collectAvailableRPMPackageUpdateCount(ctx, runner)
	case agent.OsFamily_OS_FAMILY_APT:
		return collectAvailableAPTPackageUpdateCount(ctx, runner)
	default:
		return 0, fmt.Errorf("unsupported os family: %s", osFamily.String())
	}
}

func CollectUpgradablePackages(ctx context.Context, runner ExecRunner, osFamily agent.OsFamily) ([]*agent.Package, error) {
	switch osFamily {
	case agent.OsFamily_OS_FAMILY_RPM:
		return collectUpgradableRPMPackages(ctx, runner)
	case agent.OsFamily_OS_FAMILY_APT:
		return collectUpgradableAPTPackages(ctx, runner)
	default:
		return nil, fmt.Errorf("unsupported os family: %s", osFamily.String())
	}
}

func collectUpgradableRPMPackages(ctx context.Context, runner ExecRunner) ([]*agent.Package, error) {
	output, _ := runner.Run(ctx, "dnf", "-q", "--cacheonly", "check-update")

	items := make([]*agent.Package, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		pkg := parseRpmUpgradableLine(line)
		if pkg != nil {
			items = append(items, pkg)
		}
	}

	return items, nil
}

func collectUpgradableAPTPackages(ctx context.Context, runner ExecRunner) ([]*agent.Package, error) {
	output, err := runner.Run(ctx, "apt", "list", "--upgradable")
	if err != nil {
		return nil, nil
	}

	items := make([]*agent.Package, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		pkg := parseAptUpgradableLine(line)
		if pkg != nil {
			items = append(items, pkg)
		}
	}

	return items, nil
}

func collectAvailableRPMPackageUpdateCount(ctx context.Context, runner ExecRunner) (int32, error) {
	output, _ := runner.Run(ctx, "dnf", "-q", "--cacheonly", "check-update")
	return countPackageUpdates(string(output)), nil
}

func collectAvailableAPTPackageUpdateCount(ctx context.Context, runner ExecRunner) (int32, error) {
	output, err := runner.Run(ctx, "apt", "list", "--upgradable")
	if err != nil {
		return 0, nil
	}

	return countAptPackageUpdates(string(output)), nil
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

func parseAptUpgradableLine(line string) *agent.Package {
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

	epoch, version, release, err := parseDebianVersion(strings.TrimSpace(fields[1]))
	if err != nil {
		return nil
	}

	arch := strings.TrimSpace(fields[2])
	return &agent.Package{
		Name:       name,
		Epoch:      epoch,
		Version:    version,
		Release:    release,
		Arch:       arch,
		RepoOrigin: repoOrigin,
		Nevra:      formatPackageIdentifier(name, epoch, version, release, arch),
	}
}

func parseRpmUpgradableLine(line string) *agent.Package {
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

	dot := strings.LastIndex(nameArch, ".")
	if dot <= 0 || dot+1 >= len(nameArch) {
		return nil
	}

	name := nameArch[:dot]
	arch := nameArch[dot+1:]

	epoch := int32(0)
	versionRelease := targetVersion
	if epochPart, rest, cut := strings.Cut(targetVersion, ":"); cut {
		parsedEpoch, err := parseEpoch(epochPart)
		if err != nil {
			return nil
		}
		epoch = parsedEpoch
		versionRelease = rest
	}

	version := versionRelease
	release := ""
	if dash := strings.LastIndex(versionRelease, "-"); dash > 0 && dash+1 < len(versionRelease) {
		version = versionRelease[:dash]
		release = versionRelease[dash+1:]
	}

	return &agent.Package{
		Name:       name,
		Epoch:      epoch,
		Version:    version,
		Release:    release,
		Arch:       arch,
		RepoOrigin: repoOrigin,
		Nevra:      formatPackageIdentifier(name, epoch, version, release, arch),
	}
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
