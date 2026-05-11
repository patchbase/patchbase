package collector

import (
	"bufio"
	"sort"
	"strings"

	"github.com/spf13/afero"
	agent "go.patchbase.net/proto/agent"
)

func CollectEnabledRepos(fs afero.Fs) ([]*agent.Repo, error) {
	var repos []*agent.Repo

	directories := []string{"/etc/yum.repos.d", "/etc/dnf/repos.d"}
	for _, dirPath := range directories {
		dir, err := fs.Open(dirPath)
		if err != nil {
			continue
		}

		entries, err := dir.Readdirnames(-1)
		_ = dir.Close()
		if err != nil {
			continue
		}

		for _, name := range entries {
			if !strings.HasSuffix(name, ".repo") {
				continue
			}

			path := dirPath + "/" + name
			data, err := afero.ReadFile(fs, path)
			if err != nil {
				continue
			}

			fileRepos := parseRepoFile(string(data))
			repos = append(repos, fileRepos...)
		}
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].RepoId < repos[j].RepoId
	})

	return repos, nil
}

func parseRepoFile(contents string) []*agent.Repo {
	var repos []*agent.Repo
	var current *repoDraft

	scanner := bufio.NewScanner(strings.NewReader(contents))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}

		if line[0] == '[' && line[len(line)-1] == ']' {
			if current != nil && current.enabled && current.repoID != "" {
				if !containsRepoID(repos, current.repoID) {
					repos = append(repos, current.toProto())
				}
			}
			current = &repoDraft{repoID: line[1 : len(line)-1]}
			continue
		}

		if current == nil {
			continue
		}

		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := stripQuotes(strings.TrimSpace(line[idx+1:]))

		switch key {
		case "enabled":
			current.enabled = isTruthy(value)
		case "name":
			current.repoLabel = value
		case "baseurl":
			current.baseurl = firstLiteralURL(value)
		case "metalink":
			current.metalink = firstLiteralURL(value)
		case "mirrorlist":
			current.mirrorlist = firstLiteralURL(value)
		}
	}

	if current != nil && current.enabled && current.repoID != "" {
		if !containsRepoID(repos, current.repoID) {
			repos = append(repos, current.toProto())
		}
	}

	return repos
}

type repoDraft struct {
	repoID    string
	enabled   bool
	repoLabel string
	baseurl   string
	metalink  string
	mirrorlist string
}

func (d *repoDraft) toProto() *agent.Repo {
	return &agent.Repo{
		RepoId:     d.repoID,
		Enabled:    d.enabled,
		RepoLabel:  d.repoLabel,
		Baseurl:    d.baseurl,
		Metalink:   d.metalink,
		Mirrorlist: d.mirrorlist,
	}
}

func containsRepoID(repos []*agent.Repo, id string) bool {
	for _, r := range repos {
		if r.RepoId == id {
			return true
		}
	}
	return false
}

func isTruthy(value string) bool {
	lower := strings.ToLower(value)
	return lower == "1" || lower == "true" || lower == "yes"
}

func firstLiteralURL(value string) string {
	if strings.Contains(value, "$") {
		return ""
	}

	for _, piece := range strings.Fields(value) {
		piece = strings.Trim(piece, ",")
		if strings.HasPrefix(piece, "http://") || strings.HasPrefix(piece, "https://") {
			return piece
		}
	}
	return ""
}