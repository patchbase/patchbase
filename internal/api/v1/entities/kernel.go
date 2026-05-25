package entities

import "strings"

type KernelSeverityCounts struct {
	Critical  int `json:"critical"`
	Important int `json:"important"`
	Moderate  int `json:"moderate"`
	Low       int `json:"low"`
	Unknown   int `json:"unknown"`
}

type KernelRiskView struct {
	AdvisoryCount   int                     `json:"advisory_count"`
	PackageCount    int                     `json:"package_count"`
	CVECount        int                     `json:"cve_count"`
	SeverityCounts  KernelSeverityCounts    `json:"severity_counts"`
	LatestUpdatedAt string                  `json:"latest_updated_at"`
	Advisories      []DecisionAdvisoryGroup `json:"advisories"`
}

type HostKernelPosture struct {
	RunningKernel             string         `json:"running_kernel"`
	LatestInstalledKernel     string         `json:"latest_installed_kernel"`
	RebootWouldReduceCVECount bool           `json:"reboot_would_reduce_cve_count"`
	ActiveKernel              KernelRiskView `json:"active_kernel"`
	LatestInstalled           KernelRiskView `json:"latest_installed"`
}

func BuildKernelRiskView(decisions []DecisionItem) KernelRiskView {
	if len(decisions) == 0 {
		return KernelRiskView{
			Advisories: []DecisionAdvisoryGroup{},
		}
	}

	groups := GroupDecisionsByRemediation(decisions)
	advisories := make([]DecisionAdvisoryGroup, 0)
	for _, group := range groups {
		advisories = append(advisories, group.Advisories...)
	}

	latestUpdatedAt := ""
	totalPackages := 0
	cves := make(map[string]struct{})
	counts := KernelSeverityCounts{}

	for _, advisory := range advisories {
		totalPackages += advisory.PackageCount
		updated := displayTimestampPriority(advisory.AdvisoryUpdatedAt, advisory.ComputedAt)
		if updated > latestUpdatedAt {
			latestUpdatedAt = updated
		}
		switch normalizeSeverity(advisory.SeverityLabel) {
		case "critical":
			counts.Critical++
		case "important":
			counts.Important++
		case "moderate":
			counts.Moderate++
		case "low":
			counts.Low++
		default:
			counts.Unknown++
		}
		for _, cve := range advisory.CVEs {
			if strings.TrimSpace(cve.ID) == "" {
				continue
			}
			cves[cve.ID] = struct{}{}
		}
	}

	return KernelRiskView{
		AdvisoryCount:   len(advisories),
		PackageCount:    totalPackages,
		CVECount:        len(cves),
		SeverityCounts:  counts,
		LatestUpdatedAt: latestUpdatedAt,
		Advisories:      advisories,
	}
}
