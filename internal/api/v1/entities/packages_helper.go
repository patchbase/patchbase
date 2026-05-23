package entities

import (
	"strings"

	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/utils"
)

func GroupDecisionsByRemediation(decisions []DecisionItem) []DecisionGroup {
	if len(decisions) == 0 {
		return []DecisionGroup{}
	}

	grouped := make([]DecisionGroup, 0)
	indexByFamily := make(map[string]int, len(decisions))
	advisoryIndexByFamily := make(map[string]map[string]int, len(decisions))

	for _, item := range decisions {
		familyKey := item.FamilyLabel
		index, exists := indexByFamily[familyKey]
		if !exists {
			grouped = append(grouped, DecisionGroup{
				FamilyLabel:     item.FamilyLabel,
				SeverityLabel:   item.SeverityLabel,
				SeverityTone:    item.SeverityTone,
				ActionLabel:     item.ActionLabel,
				ActionTone:      item.ActionTone,
				LatestUpdatedAt: fallback(item.AdvisoryUpdatedAt, item.ComputedAt),
				AdvisoryCount:   0,
				PackageCount:    0,
				Advisories:      []DecisionAdvisoryGroup{},
			})
			index = len(grouped) - 1
			indexByFamily[familyKey] = index
			advisoryIndexByFamily[familyKey] = map[string]int{}
		}

		group := &grouped[index]
		group.PackageCount++
		if actionPriorityLabel(item.ActionLabel) > actionPriorityLabel(group.ActionLabel) {
			group.ActionLabel = item.ActionLabel
			group.ActionTone = item.ActionTone
		}
		if severityPriorityLabel(item.SeverityLabel) > severityPriorityLabel(group.SeverityLabel) {
			group.SeverityLabel = item.SeverityLabel
			group.SeverityTone = item.SeverityTone
		}
		if displayTimestampPriority(item.AdvisoryUpdatedAt, item.ComputedAt) > displayTimestampPriority(group.LatestUpdatedAt, "") {
			group.LatestUpdatedAt = fallback(item.AdvisoryUpdatedAt, item.ComputedAt)
		}

		advisoryIndex, advisoryExists := advisoryIndexByFamily[familyKey][item.AdvisoryID]
		if !advisoryExists {
			group.Advisories = append(group.Advisories, DecisionAdvisoryGroup{
				AdvisoryID:           item.AdvisoryID,
				Title:                item.Title,
				SeverityLabel:        item.SeverityLabel,
				SeverityTone:         item.SeverityTone,
				ActionLabel:          item.ActionLabel,
				ActionTone:           item.ActionTone,
				EvidenceTier:         item.EvidenceTier,
				ComputedAt:           item.ComputedAt,
				AdvisorySourceSystem: item.AdvisorySourceSystem,
				AdvisoryURL:          item.AdvisoryURL,
				AdvisoryUpdatedAt:    item.AdvisoryUpdatedAt,
				PackageCount:         0,
				Items:                []DecisionItem{},
			})
			advisoryIndex = len(group.Advisories) - 1
			advisoryIndexByFamily[familyKey][item.AdvisoryID] = advisoryIndex
			group.AdvisoryCount++
		}

		advisory := &group.Advisories[advisoryIndex]
		advisory.Items = append(advisory.Items, item)
		advisory.PackageCount++
		if actionPriorityLabel(item.ActionLabel) > actionPriorityLabel(advisory.ActionLabel) {
			advisory.ActionLabel = item.ActionLabel
			advisory.ActionTone = item.ActionTone
		}
		if severityPriorityLabel(item.SeverityLabel) > severityPriorityLabel(advisory.SeverityLabel) {
			advisory.SeverityLabel = item.SeverityLabel
			advisory.SeverityTone = item.SeverityTone
		}
	}

	return grouped
}

func MapDecisionRow(row sql.ListDecisionPageRowsBySnapshotRow, sourceRPMs map[string]string) DecisionItem {
	severity := row.Severity.UnwrapOr("")
	sourceRPM := sourceRPMs[row.PackageName]

	return DecisionItem{
		AdvisoryID:           row.AdvisoryID,
		Title:                fallback(row.AdvisorySummary.UnwrapOr(""), row.AdvisoryID),
		FamilyLabel:          packageFamilyLabel(row.PackageName, sourceRPM),
		PackageName:          row.PackageName,
		InstalledNevra:       displayNullablePackageBuild(row.PackageName, row.InstalledNevra, "not captured"),
		FixedNevra:           displayNullablePackageBuild(row.PackageName, row.FixedNevra, "no fixed package recorded"),
		PackageStateLabel:    packageStateLabel(row.Status, row.Action),
		PackageStateTone:     packageStateTone(row.Status, row.Action),
		PackageStateIcon:     packageStateIcon(row.Status, row.Action),
		SeverityLabel:        severityLabel(severity),
		SeverityTone:         severityTone(severity),
		StatusLabel:          statusLabel(row.Status),
		ActionLabel:          actionLabel(row.Action, true),
		ActionTone:           actionTone(row.Action, true),
		EvidenceTier:         evidenceTierLabel(row.EvidenceTier),
		ReasonText:           fallback(row.ReasonText.UnwrapOr(""), row.ReasonCode),
		ComputedAt:           row.ComputedAt,
		AdvisorySourceSystem: advisorySourceLabel(row.AdvisorySourceSystem),
		AdvisoryURL:          row.AdvisorySourceUrl.UnwrapOr(""),
		AdvisoryUpdatedAt:    row.AdvisoryUpdatedAt.UnwrapOr(""),
	}
}

func packageFamilyLabel(packageName string, sourceRPM string) string {
	if sourcePackage := sourceRPMName(sourceRPM); sourcePackage != "" {
		return sourcePackage
	}

	name := strings.TrimSpace(packageName)
	if name == "" {
		return "packages"
	}

	parts := strings.Split(name, "-")
	return parts[0]
}

func sourceRPMName(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimSuffix(trimmed, ".src.rpm")
	trimmed = strings.TrimSuffix(trimmed, ".nosrc.rpm")
	if trimmed == "" {
		return ""
	}

	parts := strings.Split(trimmed, "-")
	for index, part := range parts {
		if index == 0 {
			continue
		}
		if startsWithDigit(part) {
			return strings.Join(parts[:index], "-")
		}
	}

	return trimmed
}

func startsWithDigit(value string) bool {
	if value == "" {
		return false
	}

	first := value[0]
	return first >= '0' && first <= '9'
}

func displayNullablePackageBuild(packageName string, value utils.Option[string], fallbackValue string) string {
	if !value.IsPresent() {
		return fallbackValue
	}

	return displayPackageBuild(packageName, value.UnwrapOr(""))
}

func displayRPMIdentifier(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}

	if strings.HasPrefix(trimmed, "0:") {
		return strings.TrimPrefix(trimmed, "0:")
	}

	marker := "-0:"
	index := strings.Index(trimmed, marker)
	if index <= 0 {
		return trimmed
	}

	return trimmed[:index+1] + trimmed[index+len(marker):]
}

func displayPackageBuild(packageName string, value string) string {
	trimmed := displayRPMIdentifier(value)
	if trimmed == "unknown" {
		return trimmed
	}

	name := strings.TrimSpace(packageName)
	if name != "" && strings.HasPrefix(trimmed, name+"-") {
		trimmed = strings.TrimPrefix(trimmed, name+"-")
	}

	lastDot := strings.LastIndex(trimmed, ".")
	if lastDot <= 0 {
		return trimmed
	}

	suffix := trimmed[lastDot+1:]
	if !isRPMArch(suffix) {
		return trimmed
	}

	return trimmed[:lastDot]
}

func isRPMArch(value string) bool {
	switch value {
	case "x86_64", "aarch64", "noarch", "ppc64le", "s390x", "armv7hl", "i686":
		return true
	default:
		return false
	}
}

func severityLabel(severity string) string {
	if severity == "" {
		return "Unknown"
	}

	return strings.ToUpper(severity[:1]) + strings.ToLower(severity[1:])
}

func severityTone(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "danger"
	case "important":
		return "warn"
	case "moderate":
		return "info"
	case "low":
		return "ok"
	default:
		return "muted"
	}
}

func severityPriorityLabel(label string) int {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "critical":
		return 4
	case "important":
		return 3
	case "moderate":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func actionLabel(action string, found bool) string {
	if !found {
		return "Investigate"
	}

	switch action {
	case "reboot_host":
		return "Reboot required"
	case "restart_service":
		return "Service restart"
	case "update_package":
		return "Update available"
	case "investigate":
		return "Investigate"
	case "none":
		return "Clear"
	default:
		return strings.ReplaceAll(action, "_", " ")
	}
}

func actionPriorityLabel(label string) int {
	switch label {
	case "Reboot required":
		return 4
	case "Service restart":
		return 3
	case "Update available":
		return 2
	case "Investigate":
		return 1
	default:
		return 0
	}
}

func actionTone(action string, found bool) string {
	if !found {
		return "muted"
	}

	switch action {
	case "reboot_host":
		return "danger"
	case "restart_service":
		return "warn"
	case "update_package":
		return "info"
	case "none":
		return "ok"
	default:
		return "muted"
	}
}

func packageStateLabel(status string, action string) string {
	switch action {
	case "reboot_host":
		return "Reboot required"
	case "restart_service":
		return "Service restart"
	case "investigate":
		return "Investigate"
	}

	switch status {
	case "affected_fix_available":
		return "Fix available"
	case "affected_no_fix":
		return "No fix available"
	default:
		return "Update available"
	}
}

func packageStateTone(status string, action string) string {
	switch action {
	case "reboot_host":
		return "danger"
	case "restart_service":
		return "warn"
	case "investigate":
		return "muted"
	}

	switch status {
	case "affected_no_fix":
		return "warn"
	case "affected_fix_available":
		return "info"
	default:
		return "info"
	}
}

func packageStateIcon(status string, action string) string {
	switch action {
	case "reboot_host":
		return "fa-solid fa-power-off"
	case "restart_service":
		return "fa-solid fa-rotate"
	case "investigate":
		return "fa-solid fa-circle-question"
	}

	switch status {
	case "affected_no_fix":
		return "fa-solid fa-ban"
	case "affected_fix_available":
		return "fa-solid fa-wrench"
	default:
		return "fa-solid fa-bolt"
	}
}

func evidenceTierLabel(value string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(value, "_", " "))
	if normalized == "" {
		return ""
	}

	return strings.ToUpper(normalized[:1]) + normalized[1:]
}

func advisorySourceLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "alma_errata_json":
		return "AlmaLinux"
	case "rocky_errata_api":
		return "Rocky"
	case "redhat_csaf":
		return "Red Hat"
	default:
		normalized := strings.TrimSpace(strings.ReplaceAll(value, "_", " "))
		if normalized == "" {
			return ""
		}
		return strings.ToUpper(normalized[:1]) + normalized[1:]
	}
}

func statusLabel(status string) string {
	return strings.ReplaceAll(status, "_", " ")
}

func fallback(value string, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}

	return value
}

func displayTimestampPriority(primary string, fallbackValue string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}

	return fallbackValue
}
