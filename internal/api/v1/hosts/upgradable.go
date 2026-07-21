// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package hosts

import (
	"errors"
	"net/http"

	"github.com/samber/do/v2"
	agentpb "go.patchbase.net/proto/agent"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
	"google.golang.org/protobuf/proto"
)

func GetUpgradablePackages(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)
	queries := do.MustInvoke[sql.Querier](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		hostID := r.PathValue("hostID")
		if hostID == "" {
			webutil.WriteError(w, r, apperr.ErrMissingHostID)
			return
		}

		if _, err := hostsService.GetHost(r.Context(), hostID); err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		snapshot, err := hostsService.GetLatestSnapshot(r.Context(), hostID)
		if err != nil {
			if errors.Is(err, apperr.ErrSnapshotNotFound) {
				webutil.WriteJSON(w, http.StatusOK, []entities.DecisionGroup{})
				return
			}
			webutil.WriteError(w, r, err)
			return
		}

		var agentSnap agentpb.AgentSnapshot
		sourceRPMs := make(map[string]string)
		if len(snapshot.Payload) > 0 {
			if err := proto.Unmarshal(snapshot.Payload, &agentSnap); err == nil {
				for _, p := range agentSnap.GetPackages() {
					sourceRPMs[p.GetName()] = p.GetSourceRpm()
				}
			}
		}

		rows, err := queries.ListDecisionPageRowsBySnapshot(r.Context(), snapshot.ID)
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		decisions := make([]entities.DecisionItem, 0)
		advisoryBackedPackages := make(map[string]struct{})
		for _, row := range rows {
			if !row.AdvisoryIsSecurity && row.Action != "none" {
				decisions = append(decisions, entities.MapDecisionRow(row, sourceRPMs))
				advisoryBackedPackages[row.PackageName] = struct{}{}
			}
		}
		decisions = append(
			decisions,
			entities.MapObservedUpgradablePackages(
				agentSnap.GetUpgradablePackages(),
				agentSnap.GetPackages(),
				advisoryBackedPackages,
				snapshot.CollectedAt,
			)...,
		)

		groups := entities.GroupDecisionsByRemediation(decisions)
		webutil.WriteJSON(w, http.StatusOK, groups)
	}
}
