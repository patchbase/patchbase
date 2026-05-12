package api

import (
	"fmt"
	"net/http"

	"github.com/samber/do/v2"
	apiauth "go.patchbase.net/server/internal/api/auth"
	authv1 "go.patchbase.net/server/internal/api/v1/auth"
	"go.patchbase.net/server/internal/api/v1/health"
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
	mux.HandleFunc("/api/", http.NotFound)
	mux.Handle("/", dashboardHandler)

	return mux, nil
}
