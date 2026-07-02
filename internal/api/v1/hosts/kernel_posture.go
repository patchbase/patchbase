package hosts

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/samber/do/v2"
	agentpb "go.patchbase.net/proto/agent"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
	"google.golang.org/protobuf/proto"
)

func GetKernelPosture(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)
	queries := do.MustInvoke[sql.Querier](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		hostID := r.PathValue("hostID")
		if hostID == "" {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "missing host id", nil)
			return
		}

		hostInfo, err := hostsService.GetHost(r.Context(), hostID)
		if err != nil {
			if errors.Is(err, services.ErrHostNotFound) {
				webutil.WriteAPIError(w, r, http.StatusNotFound, "host not found", nil)
			} else {
				webutil.LogError(r, "get host failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to get host", nil)
			}
			return
		}

		snapshot, err := hostsService.GetLatestSnapshot(r.Context(), hostID)
		if err != nil {
			if errors.Is(err, services.ErrSnapshotNotFound) {
				webutil.WriteJSON(w, http.StatusOK, entities.HostKernelPosture{
					ActiveKernel:              entities.BuildKernelRiskView(nil),
					LatestInstalled:           entities.BuildKernelRiskView(nil),
					RunningKernel:             "",
					LatestInstalledKernel:     "",
					RebootWouldReduceCVECount: false,
				})
				return
			}
			webutil.LogError(r, "get latest host snapshot failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to get latest snapshot", nil)
			return
		}

		var agentSnap agentpb.AgentSnapshot
		sourceRPMs := make(map[string]string)
		if len(snapshot.Payload) > 0 {
			if unmarshalErr := proto.Unmarshal(snapshot.Payload, &agentSnap); unmarshalErr == nil {
				for _, p := range agentSnap.GetPackages() {
					sourceRPMs[p.GetName()] = p.GetSourceRpm()
				}
			}
		}

		rows, err := queries.ListDecisionPageRowsBySnapshot(r.Context(), snapshot.ID)
		if err != nil {
			webutil.LogError(r, "list decision page rows failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to load decisions", nil)
			return
		}

		activeKernelDecisions := make([]entities.DecisionItem, 0)
		latestKernelDecisions := make([]entities.DecisionItem, 0)
		for _, row := range rows {
			if !row.AdvisoryIsSecurity {
				continue
			}

			item := entities.MapDecisionRow(row, sourceRPMs)
			if item.FamilyLabel != "kernel" {
				continue
			}

			if row.Action != "none" {
				activeKernelDecisions = append(activeKernelDecisions, item)
			}
			if row.Action != "none" && row.Status != "fixed_package_installed_pending_activation" {
				latestKernelDecisions = append(latestKernelDecisions, item)
			}
		}

		activeKernel := entities.BuildKernelRiskView(activeKernelDecisions)
		latestInstalled := entities.BuildKernelRiskView(latestKernelDecisions)
		osFamily := hostInfo.OSFamily
		if strings.TrimSpace(osFamily) == "" || strings.EqualFold(osFamily, "unknown") {
			osFamily = strings.ToLower(agentSnap.GetHost().GetOsFamily().String())
			osFamily = strings.TrimPrefix(osFamily, "os_family_")
		}
		posture := entities.HostKernelPosture{
			RunningKernel:             snapshot.RunningKernelNevra,
			LatestInstalledKernel:     detectLatestInstalledKernel(osFamily, snapshot.RunningKernelNevra, agentSnap.GetPackages()),
			RebootWouldReduceCVECount: activeKernel.CVECount > latestInstalled.CVECount,
			ActiveKernel:              activeKernel,
			LatestInstalled:           latestInstalled,
		}

		webutil.WriteJSON(w, http.StatusOK, posture)
	}
}

func detectLatestInstalledKernel(osFamily string, runningKernel string, packages []*agentpb.Package) string {
	switch strings.ToLower(strings.TrimSpace(osFamily)) {
	case "apt":
		return latestInstalledKernelAPT(runningKernel, packages)
	default:
		return latestInstalledKernelRPM(packages)
	}
}

func latestInstalledKernelAPT(runningKernel string, packages []*agentpb.Package) string {
	runningFlavor := kernelFlavor(runningKernel)
	best := (*agentpb.Package)(nil)

	for _, pkg := range packages {
		if !isVersionedAPTKernelPackage(pkg.GetName()) {
			continue
		}
		if runningFlavor != "" && kernelFlavor(pkg.GetName()) != runningFlavor {
			continue
		}

		if best == nil || compareAPTKernelPackageName(pkg.GetName(), best.GetName()) > 0 {
			best = pkg
		}
	}

	if best == nil && runningFlavor != "" {
		for _, pkg := range packages {
			if !isVersionedAPTKernelPackage(pkg.GetName()) {
				continue
			}
			if best == nil || compareAPTKernelPackageName(pkg.GetName(), best.GetName()) > 0 {
				best = pkg
			}
		}
	}

	if best == nil {
		return ""
	}
	return packageNEVRA(best)
}

func latestInstalledKernelRPM(packages []*agentpb.Package) string {
	best := (*agentpb.Package)(nil)
	bestRank := -1

	for _, pkg := range packages {
		name := pkg.GetName()
		if name == "" || (!strings.HasPrefix(name, "kernel") && name != "kernel") {
			continue
		}
		rank := rpmKernelNameRank(name)
		if best == nil {
			best = pkg
			bestRank = rank
			continue
		}

		if rank > bestRank {
			best = pkg
			bestRank = rank
			continue
		}
		if rank < bestRank {
			continue
		}

		current := compareKernelEVR(pkg, best)
		if current > 0 {
			best = pkg
			bestRank = rank
		}
	}

	if best == nil {
		return ""
	}
	return packageNEVRA(best)
}

func rpmKernelNameRank(name string) int {
	switch name {
	case "kernel":
		return 3
	case "kernel-core":
		return 2
	case "kernel-modules":
		return 1
	default:
		return 0
	}
}

func compareKernelEVR(left *agentpb.Package, right *agentpb.Package) int {
	leftEpoch := left.GetEpoch()
	rightEpoch := right.GetEpoch()
	if leftEpoch != rightEpoch {
		if leftEpoch > rightEpoch {
			return 1
		}
		return -1
	}

	version := compareVersionSegments(left.GetVersion(), right.GetVersion())
	if version != 0 {
		return version
	}

	return compareVersionSegments(left.GetRelease(), right.GetRelease())
}

func compareAPTKernelPackageName(left string, right string) int {
	leftKey := aptKernelVersionKey(left)
	rightKey := aptKernelVersionKey(right)
	if leftKey == "" || rightKey == "" {
		return strings.Compare(left, right)
	}
	return compareVersionSegments(leftKey, rightKey)
}

func aptKernelVersionKey(name string) string {
	trimmed := strings.TrimPrefix(name, "linux-image-")
	trimmed = strings.TrimPrefix(trimmed, "unsigned-")
	lastDash := strings.LastIndex(trimmed, "-")
	if lastDash <= 0 {
		return ""
	}
	return trimmed[:lastDash]
}

func kernelFlavor(value string) string {
	trimmed := strings.TrimSpace(value)
	lastDash := strings.LastIndex(trimmed, "-")
	if lastDash <= 0 || lastDash == len(trimmed)-1 {
		return ""
	}
	return trimmed[lastDash+1:]
}

func isVersionedAPTKernelPackage(name string) bool {
	if !strings.HasPrefix(name, "linux-image-") {
		return false
	}
	trimmed := strings.TrimPrefix(name, "linux-image-")
	trimmed = strings.TrimPrefix(trimmed, "unsigned-")
	if trimmed == "" {
		return false
	}
	first := trimmed[0]
	return first >= '0' && first <= '9'
}

func packageNEVRA(pkg *agentpb.Package) string {
	if nevra := strings.TrimSpace(pkg.GetNevra()); nevra != "" {
		return nevra
	}

	parts := []string{pkg.GetName()}
	evr := pkg.GetVersion()
	if release := pkg.GetRelease(); release != "" {
		evr = evr + "-" + release
	}
	if evr != "" {
		if pkg.GetEpoch() > 0 {
			evr = strconv.Itoa(int(pkg.GetEpoch())) + ":" + evr
		}
		parts = append(parts, evr)
	}
	if arch := pkg.GetArch(); arch != "" {
		parts = append(parts, arch)
	}
	return strings.Join(parts, "-")
}

func compareVersionSegments(left string, right string) int {
	leftTokens := strings.FieldsFunc(left, func(r rune) bool {
		return r == '.' || r == '-' || r == '_' || r == ':'
	})
	rightTokens := strings.FieldsFunc(right, func(r rune) bool {
		return r == '.' || r == '-' || r == '_' || r == ':'
	})
	maxLen := max(len(rightTokens), len(leftTokens))
	for i := range maxLen {
		leftToken := ""
		if i < len(leftTokens) {
			leftToken = leftTokens[i]
		}
		rightToken := ""
		if i < len(rightTokens) {
			rightToken = rightTokens[i]
		}
		if leftToken == rightToken {
			continue
		}
		leftNum, leftErr := strconv.Atoi(leftToken)
		rightNum, rightErr := strconv.Atoi(rightToken)
		if leftErr == nil && rightErr == nil {
			if leftNum > rightNum {
				return 1
			}
			return -1
		}
		if leftToken > rightToken {
			return 1
		}
		return -1
	}
	return 0
}
