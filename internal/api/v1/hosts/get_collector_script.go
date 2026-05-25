package hosts

import (
	"net/http"
	"strconv"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/services"
)

func GetCollectorScript(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		script := hostsService.GetCollectorScript()
		scriptBytes := []byte(script)

		w.Header().Set("Content-Type", "text/x-shellscript")
		w.Header().Set("Content-Disposition", `attachment; filename="patchbase-collector.sh"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(scriptBytes)))
		w.WriteHeader(http.StatusOK)
		w.Write(scriptBytes) // nolint: errcheck
	}
}
