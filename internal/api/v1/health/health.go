package health

import (
	"net/http"
	"time"

	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/buildinfo"
)

func Health(w http.ResponseWriter, r *http.Request) {
	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"service":   "patchbase",
		"version":   buildinfo.Version,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
