// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package matchers

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do/v2"
	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/events"
	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	"go.patchbase.net/server/internal/utils"
	"google.golang.org/protobuf/proto"
)

type Matcher interface {
	MatchSnapshot(ctx context.Context, hostID string, snapshotID string) (MatchResult, error)
	MatchHostsForScope(ctx context.Context, scopeKey string) error
}

type MatchResult struct {
	HostID            string
	SnapshotID        string
	DecisionCount     int
	OverallAction     string
	ResolvedStreamIDs []string
}

type matcher struct {
	logger  *slog.Logger
	pool    *pgxpool.Pool
	queries sql.Querier
	broker  events.Broker
}

func NewMatcher(i do.Injector) (Matcher, error) {
	logger, err := do.Invoke[*slog.Logger](i)
	if err != nil {
		return nil, err
	}
	pool, err := do.Invoke[*pgxpool.Pool](i)
	if err != nil {
		return nil, err
	}
	queries, err := do.Invoke[sql.Querier](i)
	if err != nil {
		return nil, err
	}
	broker, err := do.Invoke[events.Broker](i)
	if err != nil {
		return nil, err
	}
	return &matcher{
		logger:  logger.With("source", "MatcherService"),
		pool:    pool,
		queries: queries,
		broker:  broker,
	}, nil
}

type advisoryData struct {
	advisories          []sql.Advisory
	advisoryStreamLinks []sql.AdvisoryProductStream
	rules               []sql.AffectedPackageRule
	fixes               []sql.FixedPackage
	references          []sql.AdvisoryReference
}

func (m *matcher) loadAdvisoryDataForStreams(ctx context.Context, queries *sql.Queries, ids []string) (advisoryData, error) {
	advisories, err := queries.ListAdvisoriesByStreamIDs(ctx, ids)
	if err != nil {
		return advisoryData{}, fmt.Errorf("list advisories by stream ids: %w", err)
	}

	links, err := queries.ListAdvisoryProductStreamsByStreamIDs(ctx, ids)
	if err != nil {
		return advisoryData{}, fmt.Errorf("list advisory product streams by stream ids: %w", err)
	}

	rules, err := queries.ListAffectedPackageRulesByStreamIDs(ctx, ids)
	if err != nil {
		return advisoryData{}, fmt.Errorf("list affected package rules by stream ids: %w", err)
	}

	fixes, err := queries.ListFixedPackagesByStreamIDs(ctx, ids)
	if err != nil {
		return advisoryData{}, fmt.Errorf("list fixed packages by stream ids: %w", err)
	}

	references, err := queries.ListAdvisoryReferencesByStreamIDs(ctx, ids)
	if err != nil {
		return advisoryData{}, fmt.Errorf("list advisory references by stream ids: %w", err)
	}

	return advisoryData{
		advisories:          advisories,
		advisoryStreamLinks: links,
		rules:               rules,
		fixes:               fixes,
		references:          references,
	}, nil
}

func (m *matcher) MatchSnapshot(ctx context.Context, hostID string, snapshotID string) (MatchResult, error) {
	startedAt := time.Now()

	snapshot, err := m.queries.GetHostSnapshot(ctx, snapshotID)
	if err != nil {
		return MatchResult{}, fmt.Errorf("get host snapshot %s: %w", snapshotID, err)
	}

	host, err := m.queries.GetHostByID(ctx, hostID)
	if err != nil {
		return MatchResult{}, fmt.Errorf("get host %s: %w", hostID, err)
	}

	var agentSnap agentpb.AgentSnapshot
	if err := proto.Unmarshal(snapshot.Payload, &agentSnap); err != nil {
		return MatchResult{}, fmt.Errorf("unmarshal agent snapshot: %w", err)
	}

	productStreams, err := m.queries.ListProductStreams(ctx)
	if err != nil {
		return MatchResult{}, fmt.Errorf("list product streams: %w", err)
	}

	repos := agentSnap.GetRepos()
	packages := agentSnap.GetPackages()

	resolvedStreams := resolveProductStreams(host, repos, productStreams)
	resolvedStreamIDs := streamIDs(resolvedStreams)

	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MatchResult{}, fmt.Errorf("begin matcher transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	queries := sql.New(tx)

	if err := queries.LockHost(ctx, hostID); err != nil {
		return MatchResult{}, fmt.Errorf("lock host: %w", err)
	}

	if err := queries.DeleteDecisionRecordsBySnapshot(ctx, snapshotID); err != nil {
		return MatchResult{}, fmt.Errorf("delete decision records by snapshot: %w", err)
	}

	computedAt := time.Now().UTC().Format(time.RFC3339)

	var availablePackageUpdateCount int32
	if agentSnap.GetHost() != nil {
		availablePackageUpdateCount = agentSnap.GetHost().GetAvailablePackageUpdateCount()
	}

	if len(resolvedStreams) == 0 {
		state := m.aggregateHostCurrentState(snapshot, nil, availablePackageUpdateCount)
		state.OverallAction = "investigate"
		if err := queries.UpsertHostCurrentState(ctx, state); err != nil {
			return MatchResult{}, fmt.Errorf("upsert host current state: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return MatchResult{}, fmt.Errorf("commit matcher transaction: %w", err)
		}

		m.broker.Publish(events.NewHostMatchedEvent(hostID))

		return MatchResult{
			HostID:            hostID,
			SnapshotID:        snapshotID,
			DecisionCount:     0,
			OverallAction:     state.OverallAction,
			ResolvedStreamIDs: nil,
		}, nil
	}

	data, err := m.loadAdvisoryDataForStreams(ctx, queries, resolvedStreamIDs)
	if err != nil {
		return MatchResult{}, err
	}

	advisoryStreams := indexAdvisoryStreams(data.advisoryStreamLinks)
	rulesByAdvisory := indexRulesByAdvisory(data.rules)
	fixesByAdvisory := indexFixesByAdvisory(data.fixes)
	referencesByAdvisory := indexReferencesByAdvisory(data.references)

	decisions, err := buildDecisions(host.ID, host.OsFamily, snapshot, packages, resolvedStreams, data.advisories, advisoryStreams, rulesByAdvisory, fixesByAdvisory, referencesByAdvisory, computedAt)
	if err != nil {
		return MatchResult{}, fmt.Errorf("build decisions: %w", err)
	}

	for _, dec := range decisions {
		if _, err := tx.Exec(ctx, "SAVEPOINT decision_insert"); err != nil {
			return MatchResult{}, fmt.Errorf("create decision insert savepoint: %w", err)
		}

		if err := queries.InsertDecisionRecord(ctx, dec.record); err != nil {
			if sql.IsForeignKeyViolation(err, "decision_records_advisory_id_fkey") {
				if _, rbErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT decision_insert"); rbErr != nil {
					return MatchResult{}, fmt.Errorf("rollback decision insert savepoint: %w", rbErr)
				}
				if _, relErr := tx.Exec(ctx, "RELEASE SAVEPOINT decision_insert"); relErr != nil {
					return MatchResult{}, fmt.Errorf("release decision insert savepoint: %w", relErr)
				}
				m.logger.WarnContext(
					ctx,
					"skipping decision record insertion due to concurrently deleted advisory",
					"host_id", hostID,
					"snapshot_id", snapshotID,
					"advisory_id", dec.record.AdvisoryID,
				)
				continue
			}
			return MatchResult{}, fmt.Errorf("insert decision record: %w", err)
		}

		if _, err := tx.Exec(ctx, "RELEASE SAVEPOINT decision_insert"); err != nil {
			return MatchResult{}, fmt.Errorf("release decision insert savepoint: %w", err)
		}
	}

	state := m.aggregateHostCurrentState(snapshot, decisions, availablePackageUpdateCount)
	if err := queries.UpsertHostCurrentState(ctx, state); err != nil {
		return MatchResult{}, fmt.Errorf("upsert host current state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return MatchResult{}, fmt.Errorf("commit matcher transaction: %w", err)
	}

	m.logger.InfoContext(
		ctx,
		"matched host snapshot",
		"host_id", hostID,
		"snapshot_id", snapshotID,
		"resolved_stream_count", len(resolvedStreams),
		"decision_count", len(decisions),
		"overall_action", state.OverallAction,
		"match_ms", time.Since(startedAt).Milliseconds(),
	)

	m.broker.Publish(events.NewHostMatchedEvent(hostID))

	return MatchResult{
		HostID:            hostID,
		SnapshotID:        snapshotID,
		DecisionCount:     len(decisions),
		OverallAction:     state.OverallAction,
		ResolvedStreamIDs: resolvedStreamIDs,
	}, nil
}

func (m *matcher) MatchHostsForScope(ctx context.Context, scopeKey string) error {
	hosts, err := m.queries.ListHostsByAdvisoryScopeKey(ctx, utils.Some(scopeKey))
	if err != nil {
		return fmt.Errorf("list hosts by advisory scope key %s: %w", scopeKey, err)
	}

	for _, host := range hosts {
		if host.LastSnapshotID.IsNoneOrDefault() {
			continue
		}
		snapshotID := host.LastSnapshotID.UnwrapOr("")
		if _, err := m.MatchSnapshot(ctx, host.ID, snapshotID); err != nil {
			m.logger.WarnContext(ctx, "failed to rematch host after scope sync", "host_id", host.ID, "snapshot_id", snapshotID, "error", err)
		}
	}

	m.broker.Publish(events.NewHostsUpdatedEvent())

	return nil
}

func (m *matcher) aggregateHostCurrentState(
	snapshot sql.HostSnapshot,
	decisions []decision,
	availableUpdates int32,
) sql.UpsertHostCurrentStateParams {
	state := sql.UpsertHostCurrentStateParams{
		HostID:           snapshot.HostID,
		SnapshotID:       snapshot.ID,
		OverallAction:    "none",
		CriticalCount:    0,
		ImportantCount:   0,
		ModerateCount:    0,
		ActionableCount:  0,
		AvailableUpdates: availableUpdates,
		NeedsReboot:      0,
		NeedsRestart:     0,
		NoFix:            0,
		Unknown:          0,
	}

	for _, dec := range decisions {
		if dec.record.Status == "resolved" {
			continue
		}

		severity := normalizeSeverity(dec.severity)
		switch severity {
		case "critical":
			state.CriticalCount++
		case "important":
			state.ImportantCount++
		case "moderate":
			state.ModerateCount++
		}

		switch dec.record.Status {
		case "affected_fix_available":
			state.ActionableCount++
		case "fixed_package_installed_pending_activation":
			state.ActionableCount++
			switch dec.record.Action {
			case "reboot_host":
				state.NeedsReboot++
			case "restart_service":
				state.NeedsRestart++
			}
		case "affected_no_fix":
			state.NoFix++
		case "unknown":
			state.Unknown++
		}
	}

	state.OverallAction = overallAction(decisions)
	return state
}

type decision struct {
	record   sql.InsertDecisionRecordParams
	severity string
}

type decisionKey struct {
	installedPackageID string
	packageName        string
	installedNevra     string
}

func buildDecisions(
	hostID string,
	osFamily string,
	snapshot sql.HostSnapshot,
	packages []*agentpb.Package,
	streams []sql.ProductStream,
	advisories []sql.Advisory,
	advisoryStreams map[string][]string,
	rulesByAdvisory map[string][]sql.AffectedPackageRule,
	fixesByAdvisory map[string][]sql.FixedPackage,
	referencesByAdvisory map[string][]sql.AdvisoryReference,
	computedAt string,
) ([]decision, error) {
	resolvedStreamIDs := streamIDSet(streams)
	packagesByName := indexPackagesByName(packages)
	decisions := make([]decision, 0)

	for _, advisory := range advisories {
		if !appliesToResolvedStream(advisory.ID, resolvedStreamIDs, advisoryStreams) {
			continue
		}

		effectiveSev := effectiveSeverity(advisory, referencesByAdvisory[advisory.ID])
		filteredRulesByPackage := filterRulesByPackage(rulesByAdvisory[advisory.ID], resolvedStreamIDs)
		filteredFixesByPackage := filterFixesByPackage(fixesByAdvisory[advisory.ID], resolvedStreamIDs)
		for _, pkg := range candidatePackages(packagesByName, filteredRulesByPackage, filteredFixesByPackage) {
			if !isRelevantKernelPackage(pkg, osFamily, snapshot, packages) {
				continue
			}

			fixesForPackage := matchingFixesForPackageKeys(pkg, filteredFixesByPackage)
			rulesForPackage := matchingRulesForPackageKeys(pkg, filteredRulesByPackage)

			relevantFixes := matchingFixedPackagesForPackage(pkg, fixesForPackage, osFamily)
			relevantRules := matchingRulesForPackage(pkg, rulesForPackage, osFamily)
			if len(relevantRules) == 0 {
				if len(relevantFixes) == 0 {
					continue
				}

				bestFix, err := selectBestFix(relevantFixes, osFamily)
				if err != nil {
					return nil, fmt.Errorf("select best fix for package %s: %w", pkg.GetNevra(), err)
				}

				record := decisionFromFixedPackageOnly(hostID, osFamily, snapshot, packages, advisory, pkg, bestFix, effectiveSev, computedAt)
				if record.IsPresent() {
					decisions = append(decisions, decision{record: record.Unwrap(), severity: effectiveSev})
				}
				continue
			}

			matchedRules := make([]sql.AffectedPackageRule, 0, len(relevantRules))
			for _, rule := range relevantRules {
				matched, err := matchesRule(pkg, rule, osFamily)
				if err != nil {
					return nil, fmt.Errorf("evaluate rule %s for package %s: %w", rule.ID, pkg.GetNevra(), err)
				}
				if matched {
					matchedRules = append(matchedRules, rule)
				}
			}

			if osFamily == "apt" && len(matchedRules) > 1 {
				streamRules := make(map[string]sql.AffectedPackageRule)
				for _, rule := range matchedRules {
					existing, exists := streamRules[rule.ProductStreamID]
					if !exists {
						streamRules[rule.ProductStreamID] = rule
						continue
					}
					// Prefer binary over source
					if existing.Arch.UnwrapOr("") == "source" && rule.Arch.UnwrapOr("") == "binary" {
						streamRules[rule.ProductStreamID] = rule
						continue
					}
					if existing.Arch.UnwrapOr("") == rule.Arch.UnwrapOr("") {
						v1 := ruleVersionPart(existing.RpmEvrRule.UnwrapOr(""))
						v2 := ruleVersionPart(rule.RpmEvrRule.UnwrapOr(""))
						evr1, err1 := parseDebianEVR(v1)
						evr2, err2 := parseDebianEVR(v2)
						if err1 == nil && err2 == nil {
							if compareDebianEVR(evr2, evr1) > 0 {
								streamRules[rule.ProductStreamID] = rule
							}
						}
					}
				}
				matchedRules = make([]sql.AffectedPackageRule, 0, len(streamRules))
				for _, rule := range streamRules {
					matchedRules = append(matchedRules, rule)
				}
			}

			if len(matchedRules) == 0 {
				if len(relevantFixes) == 0 {
					continue
				}

				bestFix, err := selectBestFix(relevantFixes, osFamily)
				if err != nil {
					return nil, fmt.Errorf("select best fix for package %s: %w", pkg.GetNevra(), err)
				}

				record := decisionFromFixedPackageOnly(hostID, osFamily, snapshot, packages, advisory, pkg, bestFix, effectiveSev, computedAt)
				if record.IsPresent() {
					decisions = append(decisions, decision{record: record.Unwrap(), severity: effectiveSev})
				}
				continue
			}

			if len(matchedRules) > 1 {
				record := newDecisionRecord(hostID, snapshot.ID, advisory, pkg, utils.None[string](), "unknown", "investigate", effectiveSev, "unknown", "package_mapping_incomplete", "multiple affected package rules matched", computedAt)
				decisions = append(decisions, decision{record: record, severity: effectiveSev})
				continue
			}

			rule := matchedRules[0]
			fixedPackages := matchingFixedPackagesForStream(pkg, rule.ProductStreamID, fixesForPackage, osFamily)
			if len(fixedPackages) == 0 {
				record := newDecisionRecord(hostID, snapshot.ID, advisory, pkg, utils.Some(rule.ProductStreamID), "affected_no_fix", "investigate", effectiveSev, rule.EvidenceTier, "vendor_fix_not_available", "vendor advisory marks package affected but no fixed package is available", computedAt)
				decisions = append(decisions, decision{record: record, severity: effectiveSev})
				continue
			}

			bestFix, err := selectBestFix(fixedPackages, osFamily)
			if err != nil {
				return nil, fmt.Errorf("select best fix for package %s: %w", pkg.GetNevra(), err)
			}

			installed := evr{epoch: int64(pkg.GetEpoch()), version: pkg.GetVersion(), release: pkg.GetRelease()}
			fixed := evr{epoch: int64(bestFix.Epoch), version: bestFix.Version, release: bestFix.Release}
			evidenceTier := decisionEvidenceTier(advisory.EvidenceTier, rule.EvidenceTier, bestFix.EvidenceTier)

			if kernelRunningSatisfiesFixedBuild(snapshot, packages, pkg.GetName(), fixed, osFamily) {
				continue
			}

			var compared int
			if osFamily == "apt" {
				compared = compareDebianEVR(installed, fixed)
			} else {
				compared = compareEVR(installed, fixed)
			}
			if compared < 0 {
				status := "affected_fix_available"
				action := "update_package"
				reasonCode := "vendor_fix_available_not_installed"
				reasonText := fmt.Sprintf("a vendor fixed package (%s) is available but not installed", bestFix.Nevra)

				// If this is a versioned kernel package, check if a newer version of the same flavor/name is already installed
				if isKernelPackage(pkg.GetName(), osFamily) {
					if osFamily == "apt" {
						if isVersionedKernelPackageAPT(pkg.GetName()) {
							flavor := "generic"
							if idx := strings.LastIndex(pkg.GetName(), "-"); idx >= 0 {
								flavor = pkg.GetName()[idx+1:]
							}
							if latest, found := latestInstalledKernelEVRAPT(packages, flavor); found && compareDebianEVR(latest, fixed) >= 0 {
								status = "fixed_package_installed_pending_activation"
								action = "reboot_host"
								reasonCode = "fixed_package_installed_kernel_not_running"
								reasonText = fmt.Sprintf("fixed kernel package is installed, but the running kernel (%s) requires a reboot to activate the fix", snapshot.RunningKernelNevra)
							}
						}
					} else {
						if latest, found := latestInstalledKernelEVRRPM(packages, pkg.GetName()); found && compareEVR(latest, fixed) >= 0 {
							status = "fixed_package_installed_pending_activation"
							action = "reboot_host"
							reasonCode = "fixed_package_installed_kernel_not_running"
							reasonText = fmt.Sprintf("fixed kernel package is installed, but the running kernel (%s) requires a reboot to activate the fix", snapshot.RunningKernelNevra)
						}
					}
				}

				record := newDecisionRecord(hostID, snapshot.ID, advisory, pkg, utils.Some(rule.ProductStreamID), status, action, effectiveSev, evidenceTier, reasonCode, reasonText, computedAt)
				record.FixedNevra = utils.Some(bestFix.Nevra)
				decisions = append(decisions, decision{record: record, severity: effectiveSev})
				continue
			}

			status := "resolved"
			action := "none"
			reasonCode := "installed_package_at_or_above_fixed_build"
			reasonText := "installed package is at or above the vendor fixed build"

			if isKernelPackage(pkg.GetName(), osFamily) {
				if !kernelRunningSatisfiesFixedBuild(snapshot, packages, pkg.GetName(), fixed, osFamily) {
					status = "fixed_package_installed_pending_activation"
					action = "reboot_host"
					reasonCode = "fixed_package_installed_kernel_not_running"
					reasonText = fmt.Sprintf("fixed kernel package is installed, but the running kernel (%s) requires a reboot to activate the fix", snapshot.RunningKernelNevra)
				}
			}

			record := newDecisionRecord(hostID, snapshot.ID, advisory, pkg, utils.Some(rule.ProductStreamID), status, action, effectiveSev, evidenceTier, reasonCode, reasonText, computedAt)
			record.FixedNevra = utils.Some(bestFix.Nevra)
			decisions = append(decisions, decision{record: record, severity: effectiveSev})
		}
	}

	return collapseSupersededDecisions(decisions, osFamily), nil
}

func decisionFromFixedPackageOnly(
	hostID string,
	osFamily string,
	snapshot sql.HostSnapshot,
	packages []*agentpb.Package,
	advisory sql.Advisory,
	pkg *agentpb.Package,
	bestFix sql.FixedPackage,
	severity string,
	computedAt string,
) utils.Option[sql.InsertDecisionRecordParams] {
	installed := evr{epoch: int64(pkg.GetEpoch()), version: pkg.GetVersion(), release: pkg.GetRelease()}
	fixed := evr{epoch: int64(bestFix.Epoch), version: bestFix.Version, release: bestFix.Release}
	evidenceTier := decisionEvidenceTier(advisory.EvidenceTier, bestFix.EvidenceTier)

	if kernelRunningSatisfiesFixedBuild(snapshot, packages, pkg.GetName(), fixed, osFamily) {
		return utils.None[sql.InsertDecisionRecordParams]()
	}

	var compared int
	if osFamily == "apt" {
		compared = compareDebianEVR(installed, fixed)
	} else {
		compared = compareEVR(installed, fixed)
	}
	if compared < 0 {
		status := "affected_fix_available"
		action := "update_package"
		reasonCode := "vendor_fix_available_not_installed"
		reasonText := fmt.Sprintf("a vendor fixed package (%s) is available but not installed", bestFix.Nevra)

		if isKernelPackage(pkg.GetName(), osFamily) {
			if osFamily == "apt" {
				if isVersionedKernelPackageAPT(pkg.GetName()) {
					flavor := "generic"
					if idx := strings.LastIndex(pkg.GetName(), "-"); idx >= 0 {
						flavor = pkg.GetName()[idx+1:]
					}
					if latest, found := latestInstalledKernelEVRAPT(packages, flavor); found && compareDebianEVR(latest, fixed) >= 0 {
						status = "fixed_package_installed_pending_activation"
						action = "reboot_host"
						reasonCode = "fixed_package_installed_kernel_not_running"
						reasonText = fmt.Sprintf("fixed kernel package is installed, but the running kernel (%s) requires a reboot to activate the fix", snapshot.RunningKernelNevra)
					}
				}
			} else {
				if latest, found := latestInstalledKernelEVRRPM(packages, pkg.GetName()); found && compareEVR(latest, fixed) >= 0 {
					status = "fixed_package_installed_pending_activation"
					action = "reboot_host"
					reasonCode = "fixed_package_installed_kernel_not_running"
					reasonText = fmt.Sprintf("fixed kernel package is installed, but the running kernel (%s) requires a reboot to activate the fix", snapshot.RunningKernelNevra)
				}
			}
		}

		record := newDecisionRecord(hostID, snapshot.ID, advisory, pkg, utils.Some(bestFix.ProductStreamID), status, action, severity, evidenceTier, reasonCode, reasonText, computedAt)
		record.FixedNevra = utils.Some(bestFix.Nevra)
		return utils.Some(record)
	}

	status := "resolved"
	action := "none"
	reasonCode := "installed_package_at_or_above_fixed_build"
	reasonText := "installed package is at or above the vendor fixed build"

	if isKernelPackage(pkg.GetName(), osFamily) {
		if !kernelRunningSatisfiesFixedBuild(snapshot, packages, pkg.GetName(), fixed, osFamily) {
			status = "fixed_package_installed_pending_activation"
			action = "reboot_host"
			reasonCode = "fixed_package_installed_kernel_not_running"
			reasonText = fmt.Sprintf("fixed kernel package is installed, but the running kernel (%s) requires a reboot to activate the fix", snapshot.RunningKernelNevra)
		}
	}

	record := newDecisionRecord(hostID, snapshot.ID, advisory, pkg, utils.Some(bestFix.ProductStreamID), status, action, severity, evidenceTier, reasonCode, reasonText, computedAt)
	record.FixedNevra = utils.Some(bestFix.Nevra)
	return utils.Some(record)
}

func newDecisionRecord(
	hostID string,
	snapshotID string,
	advisory sql.Advisory,
	pkg *agentpb.Package,
	productStreamID utils.Option[string],
	status string,
	action string,
	severity string,
	evidenceTier string,
	reasonCode string,
	reasonText string,
	computedAt string,
) sql.InsertDecisionRecordParams {
	return sql.InsertDecisionRecordParams{
		ID:                 id.New("dec"),
		HostID:             hostID,
		SnapshotID:         snapshotID,
		AdvisoryID:         advisory.ID,
		InstalledPackageID: utils.None[string](),
		ProductStreamID:    productStreamID,
		PackageName:        pkg.GetName(),
		InstalledNevra:     utils.Some(pkg.GetNevra()),
		FixedNevra:         utils.None[string](),
		Status:             status,
		Action:             action,
		Severity:           utils.Some(severity),
		EvidenceTier:       evidenceTier,
		ReasonCode:         reasonCode,
		ReasonText:         utils.Some(reasonText),
		ComputedAt:         computedAt,
	}
}

func kernelRunningSatisfiesFixedBuild(snapshot sql.HostSnapshot, packages []*agentpb.Package, packageName string, fixed evr, osFamily string) bool {
	if !isKernelPackage(packageName, osFamily) {
		return false
	}

	if osFamily == "apt" {
		runningAbi := strings.TrimSpace(snapshot.RunningKernelNevra)
		fixedAbi, _ := trimKernelPackagePrefixAPT(packageName)

		if len(fixedAbi) > 0 && isDigit(fixedAbi[0]) && len(runningAbi) > 0 && isDigit(runningAbi[0]) {
			runningAbiEVR, err1 := parseRunningKernelDebianEVR(runningAbi)
			fixedAbiEVR, err2 := parseRunningKernelDebianEVR(fixedAbi)
			if err1 == nil && err2 == nil {
				abiCompared := compareDebianEVR(runningAbiEVR, fixedAbiEVR)
				if abiCompared != 0 {
					return abiCompared > 0
				}
				runningKernel := getRunningKernelPackageEVR(packages, runningAbi)
				if runningKernel.IsPresent() {
					return compareDebianEVR(runningKernel.Unwrap(), fixed) >= 0
				}
				return false
			}
		}

		runningKernel := getRunningKernelPackageEVR(packages, runningAbi)
		if runningKernel.IsPresent() {
			return compareDebianEVR(runningKernel.Unwrap(), fixed) >= 0
		}

		runningKernelDebian, err := parseRunningKernelDebianEVR(runningAbi)
		if err != nil {
			return false
		}
		return compareDebianEVR(runningKernelDebian, fixed) >= 0
	}

	runningKernel, err := parseRunningKernelEVR(snapshot.RunningKernelNevra)
	if err != nil {
		return false
	}
	return compareEVR(runningKernel, fixed) >= 0
}

func appliesToResolvedStream(advisoryID string, resolvedStreamIDs map[string]struct{}, advisoryStreams map[string][]string) bool {
	for _, streamID := range advisoryStreams[advisoryID] {
		if _, ok := resolvedStreamIDs[streamID]; ok {
			return true
		}
	}

	return false
}

func matchingRulesForPackage(pkg *agentpb.Package, rules []sql.AffectedPackageRule, osFamily string) []sql.AffectedPackageRule {
	matched := make([]sql.AffectedPackageRule, 0)
	for _, rule := range rules {
		if !matchesPackageArch(pkg.GetArch(), rule.Arch.UnwrapOr(""), osFamily) {
			continue
		}

		matched = append(matched, rule)
	}

	return matched
}

func matchesRule(pkg *agentpb.Package, rule sql.AffectedPackageRule, osFamily string) (bool, error) {
	if !rule.RpmEvrRule.IsNoneOrDefault() {
		if osFamily == "apt" {
			return evaluateDebianEVRRule(
				evr{epoch: int64(pkg.GetEpoch()), version: pkg.GetVersion(), release: pkg.GetRelease()},
				rule.RpmEvrRule.UnwrapOr(""),
			)
		}
		return evaluateEVRRule(
			evr{epoch: int64(pkg.GetEpoch()), version: pkg.GetVersion(), release: pkg.GetRelease()},
			rule.RpmEvrRule.UnwrapOr(""),
		)
	}

	return true, nil
}

func matchingFixedPackagesForStream(pkg *agentpb.Package, productStreamID string, fixes []sql.FixedPackage, osFamily string) []sql.FixedPackage {
	matched := make([]sql.FixedPackage, 0)
	for _, fix := range fixes {
		if fix.ProductStreamID != productStreamID {
			continue
		}
		if !matchesPackageArch(pkg.GetArch(), fix.Arch.UnwrapOr(""), osFamily) {
			continue
		}

		matched = append(matched, fix)
	}

	return matched
}

func matchingFixedPackagesForPackage(pkg *agentpb.Package, fixes []sql.FixedPackage, osFamily string) []sql.FixedPackage {
	matched := make([]sql.FixedPackage, 0)
	for _, fix := range fixes {
		if !matchesPackageArch(pkg.GetArch(), fix.Arch.UnwrapOr(""), osFamily) {
			continue
		}

		matched = append(matched, fix)
	}

	return matched
}

func selectBestFix(fixes []sql.FixedPackage, osFamily string) (sql.FixedPackage, error) {
	if len(fixes) == 0 {
		return sql.FixedPackage{}, fmt.Errorf("at least one fixed package is required")
	}

	best := fixes[0]
	bestEVR := evr{epoch: int64(best.Epoch), version: best.Version, release: best.Release}
	for _, candidate := range fixes[1:] {
		candidateEVR := evr{epoch: int64(candidate.Epoch), version: candidate.Version, release: candidate.Release}
		var compared int
		if osFamily == "apt" {
			compared = compareDebianEVR(candidateEVR, bestEVR)
		} else {
			compared = compareEVR(candidateEVR, bestEVR)
		}
		if compared > 0 {
			best = candidate
			bestEVR = candidateEVR
		}
	}

	return best, nil
}

func decisionEvidenceTier(tiers ...string) string {
	hasDerived := false
	for _, tier := range tiers {
		switch strings.TrimSpace(tier) {
		case "unknown":
			return "unknown"
		case "derived":
			hasDerived = true
		}
	}

	if hasDerived {
		return "derived"
	}

	return "authoritative"
}

func streamIDs(streams []sql.ProductStream) []string {
	ids := make([]string, 0, len(streams))
	for _, stream := range streams {
		ids = append(ids, stream.ID)
	}

	return ids
}

func streamIDSet(streams []sql.ProductStream) map[string]struct{} {
	ids := make(map[string]struct{}, len(streams))
	for _, stream := range streams {
		ids[stream.ID] = struct{}{}
	}

	return ids
}

func indexPackagesByName(packages []*agentpb.Package) map[string][]*agentpb.Package {
	grouped := make(map[string][]*agentpb.Package, len(packages))
	for _, pkg := range packages {
		keys := packageMatchKeys(pkg)
		for _, key := range keys {
			grouped[key] = append(grouped[key], pkg)
		}
	}

	return grouped
}

func filterRulesByPackage(rules []sql.AffectedPackageRule, resolvedStreams map[string]struct{}) map[string][]sql.AffectedPackageRule {
	grouped := make(map[string][]sql.AffectedPackageRule)
	for _, rule := range rules {
		if _, ok := resolvedStreams[rule.ProductStreamID]; !ok {
			continue
		}

		for _, key := range keysForPackageMatch(rule.PackageName, rule.SourceRpm.UnwrapOr("")) {
			grouped[key] = append(grouped[key], rule)
		}
	}

	return grouped
}

func filterFixesByPackage(fixes []sql.FixedPackage, resolvedStreams map[string]struct{}) map[string][]sql.FixedPackage {
	grouped := make(map[string][]sql.FixedPackage)
	for _, fix := range fixes {
		if _, ok := resolvedStreams[fix.ProductStreamID]; !ok {
			continue
		}

		for _, key := range keysForPackageMatch(fix.PackageName, fix.SourceRpm.UnwrapOr("")) {
			grouped[key] = append(grouped[key], fix)
		}
	}

	return grouped
}

func candidatePackages(
	packagesByName map[string][]*agentpb.Package,
	rulesByPackage map[string][]sql.AffectedPackageRule,
	fixesByPackage map[string][]sql.FixedPackage,
) []*agentpb.Package {
	candidates := make([]*agentpb.Package, 0)
	seen := make(map[string]struct{})
	for packageName, packages := range packagesByName {
		if len(rulesByPackage[packageName]) == 0 && len(fixesByPackage[packageName]) == 0 {
			continue
		}

		for _, pkg := range packages {
			key := packageInstanceKey(pkg)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			candidates = append(candidates, pkg)
		}
	}

	return candidates
}

func packageMatchKeys(pkg *agentpb.Package) []string {
	keys := keysForPackageMatch(pkg.GetName(), pkg.GetSourceRpm())
	for _, derived := range derivedAPTSourceKeysFromBinary(pkg.GetName()) {
		if !slices.Contains(keys, derived) {
			keys = append(keys, derived)
		}
	}
	return keys
}

func keysForPackageMatch(name string, source string) []string {
	keys := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)
	add := func(raw string) {
		key := strings.TrimSpace(raw)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	add(name)
	add(source)
	return keys
}

func derivedAPTSourceKeysFromBinary(name string) []string {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return nil
	}

	if trimmed == "linux-libc-dev" || trimmed == "linux-tools-common" {
		return []string{"linux"}
	}

	prefixes := []string{
		"linux-image-",
		"linux-headers-",
		"linux-modules-",
		"linux-modules-extra-",
		"linux-tools-",
	}
	rest := ""
	for _, prefix := range prefixes {
		if after, ok := strings.CutPrefix(trimmed, prefix); ok {
			rest = after
			break
		}
	}
	if rest == "" {
		return nil
	}

	lastDash := strings.LastIndex(rest, "-")
	var flavor string
	if lastDash < 0 {
		flavor = rest
	} else if lastDash == 0 || lastDash == len(rest)-1 {
		return nil
	} else {
		flavor = rest[lastDash+1:]
	}
	switch flavor {
	case "generic", "amd64", "arm64", "armhf", "i386", "powerpc", "ppc64el", "s390x", "cloud-amd64", "cloud-arm64", "rt-amd64", "rt-arm64":
		return []string{"linux"}
	case "lowlatency":
		return []string{"linux-lowlatency"}
	case "aws":
		return []string{"linux-aws"}
	case "azure":
		return []string{"linux-azure"}
	case "gcp":
		return []string{"linux-gcp"}
	case "gke":
		return []string{"linux-gke"}
	case "kvm":
		return []string{"linux-kvm"}
	case "ibm":
		return []string{"linux-ibm"}
	case "oracle":
		return []string{"linux-oracle"}
	case "nvidia":
		return []string{"linux-nvidia"}
	case "raspi":
		return []string{"linux-raspi"}
	default:
		return nil
	}
}

func packageInstanceKey(pkg *agentpb.Package) string {
	nevra := strings.TrimSpace(pkg.GetNevra())
	if nevra != "" {
		return nevra
	}
	return strings.TrimSpace(pkg.GetName()) + "|" + strings.TrimSpace(pkg.GetVersion()) + "|" + strings.TrimSpace(pkg.GetRelease()) + "|" + strings.TrimSpace(pkg.GetArch())
}

func matchingRulesForPackageKeys(pkg *agentpb.Package, rulesByPackage map[string][]sql.AffectedPackageRule) []sql.AffectedPackageRule {
	keys := packageMatchKeys(pkg)
	rules := make([]sql.AffectedPackageRule, 0)
	seen := make(map[string]struct{})
	for _, key := range keys {
		for _, rule := range rulesByPackage[key] {
			if _, ok := seen[rule.ID]; ok {
				continue
			}
			seen[rule.ID] = struct{}{}
			rules = append(rules, rule)
		}
	}
	return rules
}

func matchingFixesForPackageKeys(pkg *agentpb.Package, fixesByPackage map[string][]sql.FixedPackage) []sql.FixedPackage {
	keys := packageMatchKeys(pkg)
	fixes := make([]sql.FixedPackage, 0)
	seen := make(map[string]struct{})
	for _, key := range keys {
		for _, fix := range fixesByPackage[key] {
			if _, ok := seen[fix.ID]; ok {
				continue
			}
			seen[fix.ID] = struct{}{}
			fixes = append(fixes, fix)
		}
	}
	return fixes
}

func isKernelPackageAPT(name string) bool {
	prefixes := []string{
		"linux-image-unsigned-",
		"linux-image-",
		"linux-modules-extra-",
		"linux-modules-",
		"linux-headers-",
		"linux-tools-",
		"linux-cloud-tools-",
		"linux-buildinfo-",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func trimKernelPackagePrefixAPT(name string) (string, bool) {
	prefixes := []string{
		"linux-image-unsigned-",
		"linux-image-",
		"linux-modules-extra-",
		"linux-modules-",
		"linux-headers-",
		"linux-tools-",
		"linux-cloud-tools-",
		"linux-buildinfo-",
	}
	for _, p := range prefixes {
		if after, ok := strings.CutPrefix(name, p); ok {
			return after, true
		}
	}
	return name, false
}

func isKernelPackage(name string, osFamily string) bool {
	if osFamily == "apt" {
		return isKernelPackageAPT(name)
	}
	return name == "kernel" || strings.HasPrefix(name, "kernel-")
}

func isRelevantKernelPackage(pkg *agentpb.Package, osFamily string, snapshot sql.HostSnapshot, packages []*agentpb.Package) bool {
	if !isKernelPackage(pkg.GetName(), osFamily) {
		return true
	}

	if osFamily == "apt" {
		runningAbi := strings.TrimSpace(snapshot.RunningKernelNevra)
		if runningAbi != "" && strings.Contains(pkg.GetName(), runningAbi) {
			return true
		}

		if isVersionedKernelPackageAPT(pkg.GetName()) {
			flavor := "generic"
			if idx := strings.LastIndex(pkg.GetName(), "-"); idx >= 0 {
				flavor = pkg.GetName()[idx+1:]
			}
			if latest, found := latestInstalledKernelEVRAPT(packages, flavor); found {
				installed := evr{epoch: int64(pkg.GetEpoch()), version: pkg.GetVersion(), release: pkg.GetRelease()}
				if compareDebianEVR(installed, latest) >= 0 {
					return true
				}
			}
		} else {
			return true
		}
	} else {
		if snapshot.RunningKernelNevra != "" {
			runningKernel, err := parseRunningKernelEVR(snapshot.RunningKernelNevra)
			if err == nil {
				installed := evr{epoch: int64(pkg.GetEpoch()), version: pkg.GetVersion(), release: pkg.GetRelease()}
				if compareEVR(installed, runningKernel) == 0 {
					return true
				}
			}
		}

		if latest, found := latestInstalledKernelEVRRPM(packages, pkg.GetName()); found {
			installed := evr{epoch: int64(pkg.GetEpoch()), version: pkg.GetVersion(), release: pkg.GetRelease()}
			if compareEVR(installed, latest) >= 0 {
				return true
			}
		}
	}

	return false
}

func overallAction(decisions []decision) string {
	priority := map[string]int{
		"none":            0,
		"investigate":     1,
		"update_package":  2,
		"restart_service": 3,
		"reboot_host":     4,
	}

	selected := "none"
	for _, dec := range decisions {
		if priority[dec.record.Action] > priority[selected] {
			selected = dec.record.Action
		}
	}

	return selected
}

func indexAdvisoryStreams(links []sql.AdvisoryProductStream) map[string][]string {
	grouped := make(map[string][]string)
	for _, link := range links {
		grouped[link.AdvisoryID] = append(grouped[link.AdvisoryID], link.ProductStreamID)
	}

	return grouped
}

func indexRulesByAdvisory(rules []sql.AffectedPackageRule) map[string][]sql.AffectedPackageRule {
	grouped := make(map[string][]sql.AffectedPackageRule)
	for _, rule := range rules {
		grouped[rule.AdvisoryID] = append(grouped[rule.AdvisoryID], rule)
	}

	return grouped
}

func indexFixesByAdvisory(fixes []sql.FixedPackage) map[string][]sql.FixedPackage {
	grouped := make(map[string][]sql.FixedPackage)
	for _, fix := range fixes {
		grouped[fix.AdvisoryID] = append(grouped[fix.AdvisoryID], fix)
	}

	return grouped
}

func indexReferencesByAdvisory(refs []sql.AdvisoryReference) map[string][]sql.AdvisoryReference {
	grouped := make(map[string][]sql.AdvisoryReference)
	for _, ref := range refs {
		grouped[ref.AdvisoryID] = append(grouped[ref.AdvisoryID], ref)
	}
	return grouped
}

func effectiveSeverity(advisory sql.Advisory, references []sql.AdvisoryReference) string {
	if !advisory.Severity.Map(strings.TrimSpace).IsNoneOrDefault() {
		return strings.ToLower(strings.TrimSpace(advisory.Severity.UnwrapOr("")))
	}

	var highest string
	highestPriority := -1
	for _, ref := range references {
		if !ref.SeverityVendor.Map(strings.TrimSpace).IsNoneOrDefault() {
			sv := strings.ToLower(strings.TrimSpace(ref.SeverityVendor.UnwrapOr("")))
			prio := severityPriority(sv)
			if prio > highestPriority {
				highestPriority = prio
				highest = sv
			}
		}
	}

	return highest
}

func collapseSupersededDecisions(decisions []decision, osFamily string) []decision {
	if len(decisions) <= 1 {
		return decisions
	}

	selected := make(map[decisionKey]decision, len(decisions))
	order := make([]decisionKey, 0, len(decisions))

	for _, candidate := range decisions {
		key := newDecisionKey(candidate.record)
		current, exists := selected[key]
		if !exists {
			selected[key] = candidate
			order = append(order, key)
			continue
		}

		if preferDecision(candidate, current, osFamily) {
			selected[key] = candidate
		}
	}

	collapsed := make([]decision, 0, len(order))
	for _, key := range order {
		collapsed = append(collapsed, selected[key])
	}

	return collapsed
}

func newDecisionKey(record sql.InsertDecisionRecordParams) decisionKey {
	return decisionKey{
		installedPackageID: record.InstalledPackageID.UnwrapOr(""),
		packageName:        record.PackageName,
		installedNevra:     record.InstalledNevra.UnwrapOr(""),
	}
}

func preferDecision(candidate decision, current decision, osFamily string) bool {
	candidatePriority := decisionPriority(candidate.record)
	currentPriority := decisionPriority(current.record)
	if candidatePriority != currentPriority {
		return candidatePriority > currentPriority
	}

	candidateFixed, candidateHasFixed := parseDecisionFixedEVR(candidate.record, osFamily)
	currentFixed, currentHasFixed := parseDecisionFixedEVR(current.record, osFamily)
	switch {
	case candidateHasFixed && currentHasFixed:
		var compared int
		if osFamily == "apt" {
			compared = compareDebianEVR(candidateFixed, currentFixed)
		} else {
			compared = compareEVR(candidateFixed, currentFixed)
		}
		if compared != 0 {
			return compared > 0
		}
	case candidateHasFixed:
		return true
	case currentHasFixed:
		return false
	}

	candidateSeverity := severityPriority(candidate.severity)
	currentSeverity := severityPriority(current.severity)
	if candidateSeverity != currentSeverity {
		return candidateSeverity > currentSeverity
	}

	return candidate.record.AdvisoryID > current.record.AdvisoryID
}

func decisionPriority(record sql.InsertDecisionRecordParams) int {
	switch record.Action {
	case "reboot_host":
		return 5
	case "restart_service":
		return 4
	case "update_package":
		return 3
	case "investigate":
		return 2
	case "none":
		return 1
	default:
		return 0
	}
}

func severityPriority(severity string) int {
	switch normalizeSeverity(severity) {
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

func normalizeSeverity(severity string) string {
	normalized := strings.ToLower(strings.TrimSpace(severity))
	switch normalized {
	case "high":
		return "important"
	case "medium":
		return "moderate"
	default:
		return normalized
	}
}

func matchesPackageArch(pkgArch string, ruleArch string, osFamily string) bool {
	normalizedRuleArch := strings.ToLower(strings.TrimSpace(ruleArch))
	if normalizedRuleArch == "" {
		return true
	}

	if strings.EqualFold(osFamily, "apt") {
		switch normalizedRuleArch {
		case "binary", "source", "all", "any":
			return true
		}
	}

	return normalizedRuleArch == strings.ToLower(strings.TrimSpace(pkgArch))
}

func parseDecisionFixedEVR(record sql.InsertDecisionRecordParams, osFamily string) (evr, bool) {
	if record.FixedNevra.Map(strings.TrimSpace).IsNoneOrDefault() {
		return evr{epoch: 0, version: "", release: ""}, false
	}

	value := strings.TrimSpace(record.FixedNevra.UnwrapOr(""))
	if osFamily == "apt" {
		parsed, err := parseDebianEVRFromNEVR(value)
		if err != nil {
			return evr{epoch: 0, version: "", release: ""}, false
		}
		return parsed, true
	}

	lastDot := strings.LastIndex(value, ".")
	if lastDot == -1 {
		return evr{epoch: 0, version: "", release: ""}, false
	}

	parsed, err := parseEVRFromNEVR(value[:lastDot])
	if err != nil {
		return evr{epoch: 0, version: "", release: ""}, false
	}

	return parsed, true
}

func isVersionedKernelPackageAPT(name string) bool {
	trimmed, ok := trimKernelPackagePrefixAPT(name)
	if !ok {
		return false
	}
	return len(trimmed) > 0 && isDigit(trimmed[0])
}

func latestInstalledKernelEVRAPT(packages []*agentpb.Package, flavor string) (evr, bool) {
	var maxEVR evr
	found := false
	for _, p := range packages {
		name := p.GetName()
		if isVersionedKernelPackageAPT(name) {
			f := "generic"
			if idx := strings.LastIndex(name, "-"); idx >= 0 {
				f = name[idx+1:]
			}
			if f == flavor {
				pEVR := evr{
					epoch:   int64(p.GetEpoch()),
					version: p.GetVersion(),
					release: p.GetRelease(),
				}
				if !found || compareDebianEVR(pEVR, maxEVR) > 0 {
					maxEVR = pEVR
					found = true
				}
			}
		}
	}
	return maxEVR, found
}

func latestInstalledKernelEVRRPM(packages []*agentpb.Package, packageName string) (evr, bool) {
	var maxEVR evr
	found := false
	for _, p := range packages {
		if p.GetName() == packageName {
			pEVR := evr{
				epoch:   int64(p.GetEpoch()),
				version: p.GetVersion(),
				release: p.GetRelease(),
			}
			if !found || compareEVR(pEVR, maxEVR) > 0 {
				maxEVR = pEVR
				found = true
			}
		}
	}
	return maxEVR, found
}

func ruleVersionPart(rule string) string {
	parts := strings.Fields(strings.TrimSpace(rule))
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}
