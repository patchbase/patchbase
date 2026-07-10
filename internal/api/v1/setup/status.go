package setup

import (
	"net/http"

	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

func Status(i do.Injector) http.HandlerFunc {
	settings := do.MustInvoke[services.Settings](i)

	return func(w http.ResponseWriter, r *http.Request) {
		status, err := settings.Status(r.Context())
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, map[string]bool{
			"completed": status.Done,
		})
	}
}
