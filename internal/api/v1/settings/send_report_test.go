// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package settings_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.patchbase.net/server/internal/mailer"
	"go.patchbase.net/server/internal/mock"
	apitesting "go.patchbase.net/server/internal/testing"
	"go.uber.org/mock/gomock"
)

func TestSendReport(t *testing.T) {
	backend := apitesting.NewBackend(
		t,
		apitesting.WithFixture(apitesting.LoadYAMLFixtures("users.yml")),
	)
	
	ctrl := gomock.NewController(t)
	m := mock.NewMockMailer(ctrl)
	
	do.Override[mailer.Mailer](backend.Injector(), func(i do.Injector) (mailer.Mailer, error) {
		return m, nil
	})

	adminToken, err := backend.IssueAccessToken(context.Background(), "u_admin")
	require.NoError(t, err)

	userToken, err := backend.IssueAccessToken(context.Background(), "u_user")
	require.NoError(t, err)

	t.Run("unauthorized", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/settings/send-report", "{}")
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("forbidden", func(t *testing.T) {
		recorder := backend.HTTPPost("/api/v1/settings/send-report", "{}", apitesting.WithBearerToken(userToken))
		assert.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("success", func(t *testing.T) {
		m.EXPECT().SendReport(gomock.Any(), []string{"admin@patchbase.local"}).Return(nil)

		recorder := backend.HTTPPost("/api/v1/settings/send-report", "{}", apitesting.WithBearerToken(adminToken))
		assert.Equal(t, http.StatusOK, recorder.Code)
	})
}
