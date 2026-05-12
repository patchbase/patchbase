package dashboard

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed dist/**
var embeddedFiles embed.FS

// Files returns the built dashboard assets rooted at dist/.
func Files() (fs.FS, error) {
	dist, err := fs.Sub(embeddedFiles, "dist")
	if err != nil {
		return nil, fmt.Errorf("sub dist fs: %w", err)
	}

	return dist, nil
}
