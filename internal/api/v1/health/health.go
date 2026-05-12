package health

import (
	"net/http"
	"time"

	"go.patchbase.net/server/internal/api/webutil"
)

func Health(w http.ResponseWriter, r *http.Request) {
	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"service":   "patchbase",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
