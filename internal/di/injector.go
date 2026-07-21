// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package di

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/api"
	"go.patchbase.net/server/internal/api/auth"
	"go.patchbase.net/server/internal/config"
	"go.patchbase.net/server/internal/events"
	"go.patchbase.net/server/internal/mailer"
	"go.patchbase.net/server/internal/queue"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/services/matchers"
	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/utils"
	"go.patchbase.net/server/internal/ws"
)

func New(ctx context.Context, cfg config.Config) do.Injector {
	logger := utils.GetLogger(ctx)
	injector := do.New()

	do.ProvideValue[config.Config](injector, cfg)
	do.ProvideValue[*slog.Logger](injector, logger)
	do.ProvideValue[utils.RandomStringGenerator](injector, utils.NewRandomStringGenerator())
	do.Provide[utils.Crypto](injector, utils.NewCrypto)
	do.Provide[*http.ServeMux](injector, api.NewMux)

	// events
	do.ProvideValue[events.Broker](injector, events.NewBroker())

	// database
	do.Provide(injector, sql.NewPGXPool)
	do.Provide(injector, sql.NewWithInjector)
	do.Provide(injector, queue.NewRiverClient)
	do.Provide(injector, queue.NewPeriodicJobManager)

	// services
	do.Provide[services.SSHPullRunner](injector, services.NewSSHPullRunner)
	do.Provide[services.Auth](injector, services.NewAuth)
	do.Provide[services.Hosts](injector, services.NewHosts)
	do.Provide[services.Settings](injector, services.NewSettings)
	do.Provide[mailer.Mailer](injector, mailer.NewMailer)
	do.Provide[services.AdvisorySyncService](injector, services.NewAdvisorySync)
	do.Provide[matchers.Matcher](injector, matchers.NewMatcher)

	// api
	do.Provide[auth.Auth](injector, auth.New)
	do.Provide[ws.Hub](injector, ws.NewHub)
	return injector
}
