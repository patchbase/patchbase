package api

import (
	"fmt"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	advisoriesv1 "go.patchbase.net/server/internal/api/v1/advisories"
	agentv1 "go.patchbase.net/server/internal/api/v1/agent"
	authv1 "go.patchbase.net/server/internal/api/v1/auth"
	"go.patchbase.net/server/internal/api/v1/health"
	hostsv1 "go.patchbase.net/server/internal/api/v1/hosts"
	setupv1 "go.patchbase.net/server/internal/api/v1/setup"
)

func NewMux(i do.Injector) (*http.ServeMux, error) {
	auth, err := do.Invoke[apiauth.Auth](i)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve auth: %w", err)
	}
	dashboardHandler, err := newDashboardHandler()
	if err != nil {
		return nil, fmt.Errorf("failed to create dashboard handler: %w", err)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", health.Health)
	mux.HandleFunc("POST /api/v1/auth/login", authv1.Login(i))
	mux.HandleFunc("GET /api/v1/setup/status", setupv1.Status(i))
	mux.HandleFunc("POST /api/v1/setup/complete", auth.Required(setupv1.Complete(i)))
	mux.HandleFunc("POST /api/v1/agent/register", agentv1.Register(i))
	mux.HandleFunc("POST /api/v1/agent/snapshots", agentv1.Snapshots(i))

	mux.HandleFunc("GET /api/v1/advisories/scopes", auth.Required(advisoriesv1.GetScopeStatuses(i)))
	mux.HandleFunc("POST /api/v1/advisories/scopes/{scopeKey}/sync", auth.Required(advisoriesv1.TriggerSync(i)))
	mux.HandleFunc("GET /api/v1/advisories/overview", auth.Required(advisoriesv1.GetOverview(i)))

	mux.HandleFunc("GET /api/v1/hosts", auth.Required(hostsv1.ListHosts(i)))
	mux.HandleFunc("GET /api/v1/hosts/pending", auth.Required(hostsv1.ListPending(i)))
	mux.HandleFunc("GET /api/v1/hosts/tokens", auth.Required(hostsv1.ListTokens(i)))
	mux.HandleFunc("POST /api/v1/hosts/tokens", auth.Required(hostsv1.CreateToken(i)))
	mux.HandleFunc("POST /api/v1/hosts/tokens/{tokenID}/revoke", auth.Required(hostsv1.RevokeToken(i)))
	mux.HandleFunc("POST /api/v1/hosts/{hostID}/approve", auth.Required(hostsv1.Approve(i)))
	mux.HandleFunc("POST /api/v1/hosts/ssh", auth.Required(hostsv1.CreateSSHHost(i)))
	mux.HandleFunc("POST /api/v1/hosts/{hostID}/onboard-ssh", auth.Required(hostsv1.OnboardSSH(i)))
	mux.HandleFunc("DELETE /api/v1/hosts/{hostID}", auth.Required(hostsv1.DeleteHost(i)))
	mux.HandleFunc("GET /api/v1/hosts/{hostID}", auth.Required(hostsv1.GetHost(i)))
	mux.HandleFunc("GET /api/v1/hosts/{hostID}/snapshot", auth.Required(hostsv1.GetLatestSnapshot(i)))
	mux.HandleFunc("GET /api/v1/hosts/{hostID}/pull-jobs", auth.Required(hostsv1.ListPullJobs(i)))
	mux.HandleFunc("/api/", http.NotFound)
	mux.Handle("/", dashboardHandler)

	return mux, nil
}
