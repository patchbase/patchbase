// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package apperr

import "net/http"

var (
	// hosts
	ErrInvalidRegistrationToken = New(http.StatusUnauthorized, "invalid_registration_token", "invalid registration token")
	ErrInvalidHostAccessToken   = New(http.StatusUnauthorized, "invalid_host_access_token", "invalid host access token")
	ErrHostNotApproved          = New(http.StatusForbidden, "host_not_approved", "host pending approval")
	ErrHostNotFound             = New(http.StatusNotFound, "host_not_found", "host not found")
	ErrTokenAlreadyRevoked      = New(http.StatusNotFound, "token_already_revoked", "registration token not found or already revoked")
	ErrInvalidSnapshotPayload   = New(http.StatusBadRequest, "invalid_snapshot_payload", "invalid snapshot payload or identity mismatch")
	ErrHostIdentityMismatch     = New(http.StatusBadRequest, "host_identity_mismatch", "snapshot host identity mismatch")
	ErrSnapshotNotFound         = New(http.StatusNotFound, "snapshot_not_found", "snapshot not found")
	ErrDuplicateHostDisplayName = New(http.StatusConflict, "duplicate_host_display_name", "a host with this display name already exists")
	ErrDuplicateSSHPullHostname = New(http.StatusConflict, "duplicate_ssh_pull_hostname", "an SSH host with this pull hostname already exists")

	// auth
	ErrInvalidCredentials          = New(http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	ErrUnauthorized                = New(http.StatusUnauthorized, "unauthorized", "invalid access token")
	ErrInitialSetupAlreadyComplete = New(http.StatusConflict, "initial_setup_already_complete", "initial setup already completed")
	ErrEmailAlreadyInUse           = New(http.StatusConflict, "email_already_in_use", "email is already in use")
	ErrEmailRequired               = New(http.StatusBadRequest, "email_required", "email is required")
	ErrCurrentPasswordRequired     = New(http.StatusBadRequest, "current_password_required", "current password is required")
	ErrCurrentPasswordInvalid      = New(http.StatusUnauthorized, "current_password_invalid", "current password is invalid")
	ErrPasswordTooShort            = New(http.StatusBadRequest, "password_too_short", "password must be at least 12 characters")

	// advisories
	ErrAdvisoryNotFound = New(http.StatusNotFound, "advisory_not_found", "advisory not found")

	// transport-level (handler context)
	ErrMissingHostID     = New(http.StatusBadRequest, "missing_host_id", "missing host id")
	ErrMissingTokenID    = New(http.StatusBadRequest, "missing_token_id", "missing token id")
	ErrMissingAdvisoryID = New(http.StatusBadRequest, "missing_advisory_id", "missing advisory id")
	ErrMissingScopeKey   = New(http.StatusBadRequest, "missing_scope_key", "missing scope key")
	ErrInvalidBody       = New(http.StatusBadRequest, "invalid_request_body", "invalid request body")
	ErrInvalidParams     = New(http.StatusBadRequest, "invalid_request_parameters", "invalid request parameters")
	ErrBodyTooLarge      = New(http.StatusRequestEntityTooLarge, "request_body_too_large", "request body too large")
	ErrBodyReadFailed    = New(http.StatusBadRequest, "body_read_failed", "failed to read request body")
	ErrMissingBearer     = New(http.StatusUnauthorized, "missing_bearer_token", "missing bearer token")
	ErrInternal          = New(http.StatusInternalServerError, "internal_error", "internal server error")

	// create ssh host request validation
	ErrDisplayNameRequired = New(http.StatusBadRequest, "display_name_required", "display name is required")
	ErrHostnameRequired    = New(http.StatusBadRequest, "hostname_required", "hostname is required")
	ErrSSHUserRequired     = New(http.StatusBadRequest, "ssh_user_required", "ssh user is required")
	ErrInvalidFrequency    = New(http.StatusBadRequest, "invalid_frequency", "invalid frequency")

	// manual host / ingest manual report
	ErrDisplayNameOrHostnameRequired = New(http.StatusBadRequest, "display_name_or_hostname_required", "display name or hostname is required")
	ErrToAddressRequired             = New(http.StatusBadRequest, "to_address_required", "to address is required")
	ErrDefaultSSHPullUserEmpty       = New(http.StatusBadRequest, "default_ssh_pull_user_empty", "default ssh pull user cannot be empty")

	// initial setup validation
	ErrNameRequired = New(http.StatusBadRequest, "name_required", "name is required")

	// forbidden — per-action messages (preserved from pre-refactor handlers).
	ErrForbiddenApproveHost        = New(http.StatusForbidden, "forbidden_approve_host", "only admins can approve hosts")
	ErrForbiddenDeleteHost         = New(http.StatusForbidden, "forbidden_delete_host", "only admins can delete hosts")
	ErrForbiddenCreateToken        = New(http.StatusForbidden, "forbidden_create_token", "only admins can create registration tokens")
	ErrForbiddenListPendingHosts   = New(http.StatusForbidden, "forbidden_list_pending_hosts", "only admins can list pending hosts")
	ErrForbiddenListTokens         = New(http.StatusForbidden, "forbidden_list_tokens", "only admins can list registration tokens")
	ErrForbiddenRevokeToken        = New(http.StatusForbidden, "forbidden_revoke_token", "only admins can revoke registration tokens")
	ErrForbiddenRunSSHPull         = New(http.StatusForbidden, "forbidden_run_ssh_pull", "only admins can run ssh pull jobs")
	ErrForbiddenCreateManualHost   = New(http.StatusForbidden, "forbidden_create_manual_host", "only admins can create manual hosts")
	ErrForbiddenOnboardSSHHost     = New(http.StatusForbidden, "forbidden_onboard_ssh_host", "only admins can onboard ssh hosts")
	ErrForbiddenCreateSSHHost      = New(http.StatusForbidden, "forbidden_create_ssh_host", "only admins can create ssh hosts")
	ErrForbiddenIngestManualReport = New(http.StatusForbidden, "forbidden_ingest_manual_report", "only admins can upload manual reports")
	ErrForbiddenAccessSettings     = New(http.StatusForbidden, "forbidden_access_settings", "only admins can access settings")
	ErrForbiddenSendReport         = New(http.StatusForbidden, "forbidden_send_report", "only admins can send report")
	ErrForbiddenTestEmail          = New(http.StatusForbidden, "forbidden_test_email", "only admins can test email")
	ErrForbiddenCompleteSetup      = New(http.StatusForbidden, "forbidden_complete_setup", "only admins can complete setup")
)
