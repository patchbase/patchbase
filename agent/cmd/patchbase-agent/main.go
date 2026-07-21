// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"log/slog"
	"os"

	"go.patchbase.net/agent/internal/cli"
)

func main() {
	if err := cli.New().Execute(); err != nil {
		slog.Default().Error("failed to execute command", "error", err)
		os.Exit(1)
	}
}
