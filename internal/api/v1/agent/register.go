package agent

import (
	"net/http"

	"github.com/samber/do/v2"
	agent "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func Register(i do.Injector) http.HandlerFunc {
	hosts := do.MustInvoke[services.Hosts](i)

	return webutil.ValidateNoAuth(func(w http.ResponseWriter, r *http.Request, req *agent.RegisterHostRequest) {
		result, err := hosts.RegisterAgentHost(r.Context(), req)
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteProto(w, http.StatusCreated, result)
	})
}
