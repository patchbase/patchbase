package hosts

import (
	"errors"
	"io"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

type ingestManualReportResponse struct {
	Status string `json:"status"`
}

func IngestManualReport(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteAPIError(w, r, http.StatusForbidden, "only admins can upload manual reports", nil)
			return
		}

		hostID := r.PathValue("hostID")
		if hostID == "" {
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "missing host ID", nil)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				webutil.WriteAPIError(w, r, http.StatusRequestEntityTooLarge, "request body too large", nil)
				return
			}
			webutil.WriteAPIError(w, r, http.StatusBadRequest, "failed to read request body", nil)
			return
		}

		err = hostsService.IngestManualReport(r.Context(), hostID, body)
		if err != nil {
			webutil.LogError(r, "ingest manual report failed", err)
			webutil.WriteAPIError(w, r, http.StatusBadRequest, err.Error(), nil)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, ingestManualReportResponse{
			Status: "success",
		})
	}
}
