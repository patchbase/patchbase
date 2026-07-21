// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package matchers

import (
	"fmt"
	"strconv"
	"strings"

	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/utils"
)

// compareDebianEVR compares two evr structs using Debian rules.
func compareDebianEVR(left evr, right evr) int {
	if left.epoch < right.epoch {
		return -1
	}
	if left.epoch > right.epoch {
		return 1
	}

	if compared := compareDebianVersionStrings(left.version, right.version); compared != 0 {
		return compared
	}

	return compareDebianVersionStrings(left.release, right.release)
}

// compareDebianVersionStrings compares two debian version components.
func compareDebianVersionStrings(left, right string) int {
	lIdx, rIdx := 0, 0
	for lIdx < len(left) || rIdx < len(right) {
		// 1. Compare non-digit segments
		lStart := lIdx
		for lIdx < len(left) && !isDigit(left[lIdx]) {
			lIdx++
		}
		lNonDigit := left[lStart:lIdx]

		rStart := rIdx
		for rIdx < len(right) && !isDigit(right[rIdx]) {
			rIdx++
		}
		rNonDigit := right[rStart:rIdx]

		if lNonDigit != rNonDigit {
			if compared := compareDebianNonDigit(lNonDigit, rNonDigit); compared != 0 {
				return compared
			}
		}

		// 2. Compare digit segments
		lStart = lIdx
		for lIdx < len(left) && isDigit(left[lIdx]) {
			lIdx++
		}
		lDigit := left[lStart:lIdx]

		rStart = rIdx
		for rIdx < len(right) && isDigit(right[rIdx]) {
			rIdx++
		}
		rDigit := right[rStart:rIdx]

		if lDigit != rDigit {
			if compared := compareDebianDigit(lDigit, rDigit); compared != 0 {
				return compared
			}
		}
	}
	return 0
}

func compareDebianNonDigit(left, right string) int {
	lIdx, rIdx := 0, 0
	for lIdx < len(left) || rIdx < len(right) {
		if lIdx >= len(left) {
			if right[rIdx] == '~' {
				return 1
			}
			return -1
		}
		if rIdx >= len(right) {
			if left[lIdx] == '~' {
				return -1
			}
			return 1
		}

		if compared := compareDebianChars(left[lIdx], right[rIdx]); compared != 0 {
			return compared
		}
		lIdx++
		rIdx++
	}
	return 0
}

func compareDebianChars(c1, c2 byte) int {
	if c1 == c2 {
		return 0
	}
	if c1 == '~' {
		return -1
	}
	if c2 == '~' {
		return 1
	}

	isL1 := isLetter(c1)
	isL2 := isLetter(c2)
	if isL1 && !isL2 {
		return -1
	}
	if !isL1 && isL2 {
		return 1
	}

	if c1 < c2 {
		return -1
	}
	return 1
}

func compareDebianDigit(left, right string) int {
	leftTrimmed := strings.TrimLeft(left, "0")
	rightTrimmed := strings.TrimLeft(right, "0")

	if len(leftTrimmed) < len(rightTrimmed) {
		return -1
	}
	if len(leftTrimmed) > len(rightTrimmed) {
		return 1
	}

	if leftTrimmed < rightTrimmed {
		return -1
	}
	if leftTrimmed > rightTrimmed {
		return 1
	}
	return 0
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// parseDebianEVR parses a debian version string into an evr.
func parseDebianEVR(value string) (evr, error) {
	epoch, version, release, err := parseDebianVersionParts(value)
	if err != nil {
		return evr{}, err
	}
	return evr{
		epoch:   int64(epoch),
		version: version,
		release: release,
	}, nil
}

func parseDebianVersionParts(value string) (int32, string, string, error) {
	version := strings.TrimSpace(value)
	if version == "" {
		return 0, "", "", fmt.Errorf("empty version")
	}

	epoch := int32(0)
	if idx := strings.Index(version, ":"); idx >= 0 {
		parsed, err := parseDebianEpoch(version[:idx])
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

func parseDebianEpoch(value string) (int32, error) {
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

func isDebianArch(s string) bool {
	switch s {
	case "amd64", "arm64", "armel", "armhf", "i386", "mips64el", "mipsel", "powerpc", "ppc64el", "riscv64", "s390x", "all", "any", "loong64", "alpha", "hppa", "ia64", "m68k", "sh4", "sparc64", "x32", "sparc", "ppc64", "mips", "mips64":
		return true
	}
	return false
}

func parseDebianEVRFromNEVR(value string) (evr, error) {
	trimmed := strings.TrimSpace(value)
	lastDot := strings.LastIndex(trimmed, ".")
	body := trimmed
	if lastDot != -1 && lastDot+1 < len(trimmed) {
		suffix := trimmed[lastDot+1:]
		if isDebianArch(suffix) {
			body = trimmed[:lastDot]
		}
	}

	before, after, ok := strings.Cut(body, ":")
	var versionReleasePart string
	epoch := int64(0)
	if ok {
		epochPart := before
		lastDash := strings.LastIndex(epochPart, "-")
		epochStr := epochPart[lastDash+1:]
		if ep, err := strconv.ParseInt(epochStr, 10, 64); err == nil {
			epoch = ep
		}
		versionReleasePart = after
	} else {
		versionStart := -1
		for i := 0; i < len(body); i++ {
			if body[i] == '-' && i+1 < len(body) && isDigit(body[i+1]) {
				versionStart = i + 1
				break
			}
		}
		if versionStart == -1 {
			for i := 0; i < len(body); i++ {
				if isDigit(body[i]) {
					versionStart = i
					break
				}
			}
		}
		if versionStart == -1 {
			return evr{}, fmt.Errorf("could not find version start in %s", value)
		}
		versionReleasePart = body[versionStart:]
	}

	lastDash := strings.LastIndex(versionReleasePart, "-")
	version := versionReleasePart
	release := ""
	if lastDash != -1 {
		version = versionReleasePart[:lastDash]
		release = versionReleasePart[lastDash+1:]
	}

	return evr{
		epoch:   epoch,
		version: version,
		release: release,
	}, nil
}

func parseRunningKernelDebianEVR(value string) (evr, error) {
	var none evr
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return none, fmt.Errorf("empty running kernel version")
	}

	before, after, ok := strings.Cut(trimmed, "-")
	if !ok {
		return evr{
			epoch:   0,
			version: trimmed,
			release: "",
		}, nil
	}

	return evr{
		epoch:   0,
		version: before,
		release: after,
	}, nil
}

func evaluateDebianEVRRule(installed evr, rule string) (bool, error) {
	parts := strings.Fields(strings.TrimSpace(rule))
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid debian evr rule: %s", rule)
	}

	target, err := parseDebianEVR(parts[1])
	if err != nil {
		return false, fmt.Errorf("parse target evr from rule %s: %w", rule, err)
	}

	compared := compareDebianEVR(installed, target)

	switch parts[0] {
	case "<":
		return compared < 0, nil
	case "<=":
		return compared <= 0, nil
	case "=":
		return compared == 0, nil
	case ">=":
		return compared >= 0, nil
	case ">":
		return compared > 0, nil
	default:
		return false, fmt.Errorf("unsupported debian evr rule operator: %s", parts[0])
	}
}

// getRunningKernelPackageEVR attempts to find the package named "linux-image-<runningKernelNevra>"
// in the installed packages list, returning its version as an evr.
func getRunningKernelPackageEVR(packages []*agentpb.Package, runningKernelNevra string) utils.Option[evr] {
	targetPkgName := "linux-image-" + strings.TrimSpace(runningKernelNevra)
	for _, pkg := range packages {
		if pkg.GetName() == targetPkgName {
			return utils.Some(evr{
				epoch:   int64(pkg.GetEpoch()),
				version: pkg.GetVersion(),
				release: pkg.GetRelease(),
			})
		}
	}
	return utils.None[evr]()
}
