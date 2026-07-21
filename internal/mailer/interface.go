// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package mailer

import (
	"context"

	"go.patchbase.net/server/internal/services"
)

type Mailer interface {
	TestConnection(ctx context.Context, s services.SMTPSettings, to string) error
	SendReport(ctx context.Context, to []string) error
}
