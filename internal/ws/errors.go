// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package ws

import "errors"

var (
	errInvalidAuthMessage         = errors.New("invalid auth message")
	errInvalidSubscriptionMessage = errors.New("invalid subscription message")
)
