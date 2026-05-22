package di

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/api"
	"go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/config"
	"go.patchbase.net/server/internal/queue"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/utils"
)

func New(ctx context.Context, cfg config.Config) do.Injector {
	logger := utils.GetLogger(ctx)
	injector := do.New()

	do.ProvideValue[config.Config](injector, cfg)
	do.ProvideValue[*slog.Logger](injector, logger)
	do.ProvideValue[utils.RandomStringGenerator](injector, utils.NewRandomStringGenerator())
	do.Provide[utils.Crypto](injector, utils.NewCrypto)
	do.Provide[*http.ServeMux](injector, api.NewMux)

	// database
	do.Provide(injector, sql.NewPGXPool)
	do.Provide(injector, sql.NewWithInjector)
	do.Provide(injector, queue.NewRiverClient)
	do.Provide(injector, queue.NewPeriodicJobManager)

	// services
	do.Provide[services.Auth](injector, services.NewAuth)
	do.Provide[services.Hosts](injector, services.NewHosts)
	do.Provide[services.Settings](injector, services.NewSettings)
	do.Provide[services.AdvisorySyncService](injector, services.NewAdvisorySync)

	// api
	do.Provide[auth.Auth](injector, auth.New)

	return injector
}
