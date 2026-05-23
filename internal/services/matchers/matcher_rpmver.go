package matchers

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type evr struct {
	epoch   int64
	version string
	release string
}

func compareEVR(left evr, right evr) int {
	switch {
	case left.epoch < right.epoch:
		return -1
	case left.epoch > right.epoch:
		return 1
	}

	if compared := compareRPMVersion(left.version, right.version); compared != 0 {
		return compared
	}

	return compareRPMVersion(left.release, right.release)
}

func compareRPMVersion(left string, right string) int {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)

	for left != "" || right != "" {
		left = trimNonAlnum(left)
		right = trimNonAlnum(right)

		switch {
		case left == "" && right == "":
			return 0
		case left == "":
			return -1
		case right == "":
			return 1
		}

		leftSegment, leftNumeric, nextLeft := nextSegment(left)
		rightSegment, rightNumeric, nextRight := nextSegment(right)

		if leftNumeric != rightNumeric {
			if leftNumeric {
				return 1
			}

			return -1
		}

		compared := compareSegment(leftSegment, rightSegment, leftNumeric)
		if compared != 0 {
			return compared
		}

		left = nextLeft
		right = nextRight
	}

	return 0
}

func evaluateEVRRule(installed evr, rule string) (bool, error) {
	parts := strings.Fields(strings.TrimSpace(rule))
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid rpm evr rule: %s", rule)
	}

	target, err := parseEVR(parts[1])
	if err != nil {
		return false, fmt.Errorf("parse target evr from rule %s: %w", rule, err)
	}

	compared := compareEVR(installed, target)

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
		return false, fmt.Errorf("unsupported rpm evr rule operator: %s", parts[0])
	}
}

func parseEVR(value string) (evr, error) {
	trimmed := strings.TrimSpace(value)
	dashIndex := strings.LastIndex(trimmed, "-")
	if dashIndex == -1 {
		return evr{}, fmt.Errorf("missing release separator in %s", value)
	}

	colonIndex := strings.Index(trimmed, ":")
	epoch := int64(0)
	versionStart := 0
	if colonIndex != -1 {
		if dashIndex < colonIndex {
			return evr{}, fmt.Errorf("missing release separator in %s", value)
		}

		parsedEpoch, err := strconv.ParseInt(trimmed[:colonIndex], 10, 64)
		if err != nil {
			return evr{}, fmt.Errorf("parse epoch from %s: %w", value, err)
		}

		epoch = parsedEpoch
		versionStart = colonIndex + 1
	}

	return evr{
		epoch:   epoch,
		version: trimmed[versionStart:dashIndex],
		release: trimmed[dashIndex+1:],
	}, nil
}

func parseRunningKernelEVR(value string) (evr, error) {
	trimmed := strings.TrimSpace(value)
	lastDot := strings.LastIndex(trimmed, ".")
	if lastDot == -1 {
		return evr{}, fmt.Errorf("missing running kernel arch in %s", value)
	}

	body := trimmed[:lastDot]
	if strings.Contains(body, ":") {
		if parsed, err := parseEVR(body); err == nil {
			return parsed, nil
		}
	}

	if parsed, err := parseEVRFromNEVR(body); err == nil {
		return parsed, nil
	}

	if parsed, err := parseEVR(body); err == nil {
		return parsed, nil
	}

	return evr{}, fmt.Errorf("invalid running kernel evr %s", value)
}

func parseEVRFromNEVR(value string) (evr, error) {
	trimmed := strings.TrimSpace(value)
	lastDash := strings.LastIndex(trimmed, "-")
	if lastDash == -1 {
		return evr{}, fmt.Errorf("missing release separator in %s", value)
	}

	release := trimmed[lastDash+1:]
	body := trimmed[:lastDash]
	if release == "" || body == "" {
		return evr{}, fmt.Errorf("missing release separator in %s", value)
	}

	colonIndex := strings.Index(body, ":")
	if colonIndex == -1 {
		versionDash := strings.LastIndex(body, "-")
		if versionDash == -1 {
			return evr{}, fmt.Errorf("missing version separator in %s", value)
		}

		version := body[versionDash+1:]
		if version == "" {
			return evr{}, fmt.Errorf("missing version separator in %s", value)
		}

		return evr{
			epoch:   0,
			version: version,
			release: release,
		}, nil
	}

	epochDash := strings.LastIndex(body[:colonIndex], "-")
	if epochDash == -1 {
		return evr{}, fmt.Errorf("missing epoch separator in %s", value)
	}

	epoch, err := strconv.ParseInt(body[epochDash+1:colonIndex], 10, 64)
	if err != nil {
		return evr{}, fmt.Errorf("parse epoch from %s: %w", value, err)
	}

	version := body[colonIndex+1:]
	if version == "" {
		return evr{}, fmt.Errorf("missing version separator in %s", value)
	}

	return evr{
		epoch:   epoch,
		version: version,
		release: release,
	}, nil
}

func trimNonAlnum(value string) string {
	for len(value) > 0 {
		r := rune(value[0])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return value
		}

		value = value[1:]
	}

	return value
}

func nextSegment(value string) (string, bool, string) {
	if value == "" {
		return "", false, ""
	}

	numeric := unicode.IsDigit(rune(value[0]))
	index := 0

	for index < len(value) {
		current := rune(value[index])
		if unicode.IsDigit(current) != numeric {
			break
		}
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			break
		}
		index++
	}

	return value[:index], numeric, value[index:]
}

func compareSegment(left string, right string, numeric bool) int {
	if numeric {
		left = strings.TrimLeft(left, "0")
		right = strings.TrimLeft(right, "0")

		switch {
		case len(left) < len(right):
			return -1
		case len(left) > len(right):
			return 1
		}
	}

	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
