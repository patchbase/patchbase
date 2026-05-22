package agent

import (
	"errors"
	"net/http"
	"strings"

	"github.com/samber/do/v2"
	agent "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func Snapshots(i do.Injector) http.HandlerFunc {
	hosts := do.MustInvoke[services.Hosts](i)

	return webutil.ValidateNoAuth(func(w http.ResponseWriter, r *http.Request, req *agent.AgentSnapshot) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="patchbase-agent"`)
			webutil.WriteAPIError(w, r, http.StatusUnauthorized, "missing bearer token", nil)
			return
		}

		result, err := hosts.IngestAgentSnapshot(r.Context(), token, req)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrInvalidHostAccessToken):
				w.Header().Set("WWW-Authenticate", `Bearer realm="patchbase-agent"`)
				webutil.WriteAPIError(w, r, http.StatusUnauthorized, "invalid host access token", nil)
			case errors.Is(err, services.ErrHostNotApproved):
				webutil.WriteAPIError(w, r, http.StatusForbidden, "host pending approval", nil)
			case errors.Is(err, services.ErrInvalidSnapshotPayload):
				webutil.WriteAPIError(w, r, http.StatusBadRequest, "invalid snapshot payload", nil)
			default:
				webutil.LogError(r, "ingest agent snapshot failed", err)
				webutil.WriteAPIError(w, r, http.StatusInternalServerError, "snapshot ingest failed", nil)
			}
			return
		}

		webutil.WriteProto(w, http.StatusAccepted, result)
	})
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	value := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if value == "" {
		return "", false
	}
	return value, true
}
