// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package utils

import "strings"

func EmptySpaceString(s string) Option[string] {
	if strings.TrimSpace(s) == "" {
		return None[string]()
	}
	return Some(s)
}
