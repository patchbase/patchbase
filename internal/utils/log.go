// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package utils

import (
	"context"
	"log/slog"
)

type loggerContextKey struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

func GetLogger(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerContextKey{}).(*slog.Logger)
	if ok && logger != nil {
		return logger
	}

	return slog.Default()
}
