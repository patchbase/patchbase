package hosts

import (
	"errors"
	"net/http"

	"github.com/samber/do/v2"
	agentpb "go.patchbase.net/proto/agent"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
	"google.golang.org/protobuf/proto"
)

func GetVulnerablePackages(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)
	queries := do.MustInvoke[sql.Querier](i)

	return func(w http.ResponseWriter, r *http.Request, _ apiauth.AuthInfo) {
		hostID := r.PathValue("hostID")
		if hostID == "" {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "missing host id", nil)
			return
		}

		_, err := hostsService.GetHost(r.Context(), hostID)
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
				webutil.WriteJSON(w, http.StatusOK, []entities.DecisionGroup{})
				return
			}
			webutil.LogError(r, "get latest host snapshot failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to get latest snapshot", nil)
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
			webutil.LogError(r, "list decision page rows failed", err)
			webutil.WriteAPIError(w, r, http.StatusInternalServerError, "failed to load decisions", nil)
			return
		}

		decisions := make([]entities.DecisionItem, 0)
		for _, row := range rows {
			if row.AdvisoryIsSecurity && row.Action != "none" {
				decisions = append(decisions, entities.MapDecisionRow(row, sourceRPMs))
			}
		}

		groups := entities.GroupDecisionsByRemediation(decisions)
		webutil.WriteJSON(w, http.StatusOK, groups)
	}
}
