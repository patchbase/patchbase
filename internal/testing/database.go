// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package testing

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	gotesting "testing"

	"github.com/jackc/pgx/v5"
	"go.patchbase.net/server/internal/utils"
)

func createEphemeralDatabase(t *gotesting.T, baseURL string) (string, error) {
	t.Helper()

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse database url: %w", err)
	}
	if parsedURL.Scheme != "postgres" && parsedURL.Scheme != "postgresql" {
		return "", fmt.Errorf("unsupported database scheme: %s", parsedURL.Scheme)
	}
	templateDB := strings.TrimPrefix(parsedURL.Path, "/")
	if templateDB == "" {
		return "", fmt.Errorf("database url missing template database name in path: %q", parsedURL.Path)
	}

	dbName := "patchbase_test_" + utils.RandomHex(8)
	adminURL := *parsedURL
	adminURL.Path = "/postgres"

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminURL.String())
	if err != nil {
		return "", fmt.Errorf("connect admin database: %w", err)
	}
	defer conn.Close(ctx) // nolint:errcheck

	createSQL := fmt.Sprintf(`CREATE DATABASE %s TEMPLATE %s`, quoteIdent(dbName), quoteIdent(templateDB))
	if _, err := conn.Exec(ctx, createSQL); err != nil {
		return "", fmt.Errorf("create ephemeral database %q from template %q: %w", dbName, templateDB, err)
	}

	testURL := *parsedURL
	testURL.Path = "/" + dbName

	t.Cleanup(func() {
		dropEphemeralDatabase(adminURL.String(), dbName)
	})

	return testURL.String(), nil
}

func dropEphemeralDatabase(adminURL string, dbName string) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return
	}
	defer conn.Close(ctx) // nolint:errcheck

	// nolint:errcheck
	conn.Exec(ctx, `
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = $1
		  AND pid <> pg_backend_pid()
	`, dbName)

	conn.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, quoteIdent(dbName))) // nolint:errcheck
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
