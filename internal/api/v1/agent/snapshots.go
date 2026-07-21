// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package agent

import (
	"errors"
	"net/http"
	"strings"

	"github.com/samber/do/v2"
	agent "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
)

func Snapshots(i do.Injector) http.HandlerFunc {
	hosts := do.MustInvoke[services.Hosts](i)

	return webutil.ValidateNoAuth(func(w http.ResponseWriter, r *http.Request, req *agent.AgentSnapshot) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="patchbase-agent"`)
			webutil.WriteError(w, r, apperr.ErrMissingBearer)
			return
		}

		result, err := hosts.IngestAgentSnapshot(r.Context(), token, req)
		if err != nil {
			if errors.Is(err, apperr.ErrInvalidHostAccessToken) ||
				errors.Is(err, apperr.ErrHostNotFound) ||
				errors.Is(err, apperr.ErrMissingBearer) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="patchbase-agent"`)
			}
			webutil.WriteError(w, r, err)
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
