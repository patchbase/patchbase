// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package audit

import (
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/api/v1/entities"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/services"
)

func List(i do.Injector) apiauth.AuthenticatedHandler {
	auditLog := do.MustInvoke[services.AuditLogService](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteError(w, r, apperr.ErrForbiddenListAuditLog)
			return
		}

		q := r.URL.Query()

		limit, err := webutil.ParseInt32Opt(q.Get("limit"), "limit")
		if err != nil {
			webutil.WriteError(w, r, apperr.WithDetails(apperr.ErrInvalidParams, err))
			return
		}
		offset, err := webutil.ParseInt32Opt(q.Get("offset"), "offset")
		if err != nil {
			webutil.WriteError(w, r, apperr.WithDetails(apperr.ErrInvalidParams, err))
			return
		}

		from, err := webutil.ParseTimestamp(q.Get("from"))
		if err != nil {
			webutil.WriteError(w, r, apperr.WithDetails(apperr.ErrInvalidParams, err))
			return
		}
		to, err := webutil.ParseTimestampEnd(q.Get("to"))
		if err != nil {
			webutil.WriteError(w, r, apperr.WithDetails(apperr.ErrInvalidParams, err))
			return
		}

		result, err := auditLog.List(r.Context(), services.ListAuditLogInput{
			Limit:  limit,
			Offset: offset,
			Action: q.Get("action"),
			Actor:  q.Get("actor"),
			From:   from,
			To:     to,
		})
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusOK, entities.NewAuditLogPage(result))
	}
}
