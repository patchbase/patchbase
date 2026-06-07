package entities

import (
	"fmt"
	"strings"
	"time"

	agentpb "go.patchbase.net/proto/agent"
)

func MapObservedUpgradablePackages(
	upgradable []*agentpb.Package,
	installedPackages []*agentpb.Package,
	advisoryBackedPackages map[string]struct{},
	collectedAt time.Time,
) []DecisionItem {
	installedByName := make(map[string]*agentpb.Package)
	installedByNameArch := make(map[string]*agentpb.Package)
	for _, pkg := range installedPackages {
		if pkg.GetName() == "" {
			continue
		}
		if _, exists := installedByName[pkg.GetName()]; !exists {
			installedByName[pkg.GetName()] = pkg
		}
		key := observedPackageKey(pkg.GetName(), pkg.GetArch())
		if _, exists := installedByNameArch[key]; !exists {
			installedByNameArch[key] = pkg
		}
	}

	items := make([]DecisionItem, 0, len(upgradable))
	computedAt := collectedAt.UTC().Format(time.RFC3339)

	for _, pkg := range upgradable {
		name := strings.TrimSpace(pkg.GetName())
		if name == "" {
			continue
		}
		if _, hasAdvisoryMatch := advisoryBackedPackages[name]; hasAdvisoryMatch {
			continue
		}

		installed := installedByNameArch[observedPackageKey(name, pkg.GetArch())]
		if installed == nil {
			installed = installedByName[name]
		}

		reason := "Reported by host package manager as upgradable."
		if repo := strings.TrimSpace(pkg.GetRepoOrigin()); repo != "" {
			reason = fmt.Sprintf("%s Repo: %s.", reason, repo)
		}

		items = append(items, DecisionItem{
			AdvisoryID:           "host-package-manager",
			Title:                "Host package manager updates",
			FamilyLabel:          packageFamilyLabel(name, ""),
			PackageName:          name,
			InstalledNevra:       observedPackageBuildOrFallback(installed, "not captured"),
			FixedNevra:           observedPackageBuildOrFallback(pkg, "target version not captured"),
			PackageStateLabel:    "Update available",
			PackageStateTone:     "info",
			PackageStateIcon:     "fa-solid fa-bolt",
			SeverityLabel:        "Unknown",
			SeverityTone:         "muted",
			StatusLabel:          "observed upgradable",
			ActionLabel:          "Update available",
			ActionTone:           "info",
			EvidenceTier:         "Host package manager",
			ReasonText:           reason,
			ComputedAt:           computedAt,
			AdvisorySourceSystem: "Host package manager",
			AdvisoryURL:          "",
			AdvisoryUpdatedAt:    computedAt,
			CVEs:                 []CVEInfo{},
		})
	}

	return items
}

func observedPackageBuildOrFallback(pkg *agentpb.Package, fallback string) string {
	if pkg == nil {
		return fallback
	}
	if nevra := strings.TrimSpace(pkg.GetNevra()); nevra != "" {
		return nevra
	}
	version := strings.TrimSpace(pkg.GetVersion())
	release := strings.TrimSpace(pkg.GetRelease())
	arch := strings.TrimSpace(pkg.GetArch())
	epoch := pkg.GetEpoch()

	switch {
	case version == "":
		return fallback
	case release != "":
		build := fmt.Sprintf("%d:%s-%s", epoch, version, release)
		if epoch == 0 {
			build = fmt.Sprintf("%s-%s", version, release)
		}
		if arch != "" {
			build = fmt.Sprintf("%s.%s", build, arch)
		}
		return build
	default:
		if arch != "" {
			return fmt.Sprintf("%s.%s", version, arch)
		}
		return version
	}
}

func observedPackageKey(name string, arch string) string {
	return strings.TrimSpace(name) + "|" + strings.TrimSpace(arch)
}
