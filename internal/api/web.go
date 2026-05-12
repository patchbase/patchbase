package api

import (
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"

	dashboard "go.patchbase.net/dashboard"
)

type dashboardHandler struct {
	files fs.FS
}

func newDashboardHandler() (http.Handler, error) {
	files, err := dashboard.Files()
	if err != nil {
		return nil, fmt.Errorf("load dashboard assets: %w", err)
	}

	return dashboardHandler{files: files}, nil
}

func (h dashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}

	requestedPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if requestedPath == "." || requestedPath == "" {
		requestedPath = "index.html"
	}

	if isFile(h.files, requestedPath) {
		http.ServeFileFS(w, r, h.files, requestedPath)
		return
	}

	http.ServeFileFS(w, r, h.files, "index.html")
}

func isFile(files fs.FS, filePath string) bool {
	info, err := fs.Stat(files, filePath)
	if err != nil {
		return false
	}

	return !info.IsDir()
}
