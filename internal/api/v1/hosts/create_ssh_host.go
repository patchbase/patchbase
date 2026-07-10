package hosts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/api/webutil"
	"go.patchbase.net/server/internal/services"
)

type createSSHHostRequest struct {
	DisplayName      string `json:"display_name"`
	Hostname         string `json:"hostname"`
	SSHUser          string `json:"ssh_user"`
	FrequencyMinutes int32  `json:"frequency_minutes"`
	UniqueKeyPair    bool   `json:"unique_key_pair"`
}

type createSSHHostResponse struct {
	HostID         string `json:"host_id"`
	PublicKey      string `json:"public_key"`
	ApprovalStatus string `json:"approval_status"`
	LastRunStatus  string `json:"last_run_status"`
	LastRunError   string `json:"last_run_error"`
}

func CreateSSHHost(i do.Injector) apiauth.AuthenticatedHandler {
	hostsService := do.MustInvoke[services.Hosts](i)

	return func(w http.ResponseWriter, r *http.Request, authInfo apiauth.AuthInfo) {
		if !authInfo.User.IsAdmin {
			webutil.WriteError(w, r, apperr.ErrForbiddenCreateSSHHost)
			return
		}

		var req createSSHHostRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			webutil.WriteError(w, r, apperr.ErrInvalidBody)
			return
		}

		settingsService := do.MustInvoke[services.Settings](i)
		if err := req.validate(r.Context(), settingsService); err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		result, err := hostsService.CreateSSHHost(r.Context(), services.CreateSSHHostInput{
			DisplayName:      req.DisplayName,
			Hostname:         req.Hostname,
			SSHUser:          req.SSHUser,
			FrequencyMinutes: req.FrequencyMinutes,
			UniqueKeyPair:    req.UniqueKeyPair,
		})
		if err != nil {
			webutil.WriteError(w, r, err)
			return
		}

		webutil.WriteJSON(w, http.StatusCreated, createSSHHostResponse{
			HostID:         result.HostID,
			PublicKey:      result.PublicKey,
			ApprovalStatus: result.ApprovalStatus,
			LastRunStatus:  result.LastRunStatus,
			LastRunError:   result.LastRunError,
		})
	}
}

func (req *createSSHHostRequest) validate(ctx context.Context, settingsService services.Settings) error {
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		return apperr.ErrDisplayNameRequired
	}

	req.Hostname = strings.TrimSpace(req.Hostname)
	if req.Hostname == "" {
		return apperr.ErrHostnameRequired
	}

	req.SSHUser = strings.TrimSpace(req.SSHUser)
	if req.SSHUser == "" {
		defaultUser, err := settingsService.GetDefaultSSHPullUser(ctx)
		if err != nil {
			return fmt.Errorf("failed to get default ssh user: %w", err)
		}
		if defaultUser != "" {
			req.SSHUser = defaultUser
		} else {
			return apperr.ErrSSHUserRequired
		}
	}

	if req.FrequencyMinutes < 0 {
		return apperr.ErrInvalidFrequency
	}

	return nil
}