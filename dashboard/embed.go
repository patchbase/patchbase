// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package dashboard

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed all:build
var embeddedFiles embed.FS

// Files returns the built dashboard assets rooted at build/.
func Files() (fs.FS, error) {
	dist, err := fs.Sub(embeddedFiles, "build")
	if err != nil {
		return nil, fmt.Errorf("sub build fs: %w", err)
	}

	return dist, nil
}
