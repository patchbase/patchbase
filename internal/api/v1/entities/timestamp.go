// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package entities

import (
	"time"
)

const apiTimeLayout = "2006-01-02T15:04:05.000000Z"

func TimeToString(t time.Time) string {
	return t.UTC().Format(apiTimeLayout)
}
