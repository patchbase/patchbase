package collector

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	agent "go.patchbase.net/proto/agent"
)

func CollectEnabledRepos(fs afero.Fs, osFamily agent.OsFamily) ([]*agent.Repo, error) {
	switch osFamily {
	case agent.OsFamily_OS_FAMILY_RPM:
		return collectEnabledRPMRepos(fs), nil
	case agent.OsFamily_OS_FAMILY_APT:
		return collectEnabledAPTRepos(fs), nil
	default:
		return nil, fmt.Errorf("unsupported os family: %s", osFamily.String())
	}
}

func collectEnabledRPMRepos(fs afero.Fs) []*agent.Repo {
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

	return repos
}

func collectEnabledAPTRepos(fs afero.Fs) []*agent.Repo {
	var repos []*agent.Repo

	repos = append(repos, parseAptListFile(string(mustReadFile(fs, "/etc/apt/sources.list")), "/etc/apt/sources.list")...)

	dir, err := fs.Open("/etc/apt/sources.list.d")
	if err == nil {
		entries, readErr := dir.Readdirnames(-1)
		_ = dir.Close()
		if readErr == nil {
			sort.Strings(entries)
			for _, name := range entries {
				path := "/etc/apt/sources.list.d/" + name
				data := mustReadFile(fs, path)
				if len(data) == 0 {
					continue
				}
				switch {
				case strings.HasSuffix(name, ".list"):
					repos = append(repos, parseAptListFile(string(data), path)...)
				case strings.HasSuffix(name, ".sources"):
					repos = append(repos, parseAptSourcesFile(string(data), path)...)
				}
			}
		}
	}

	repoByID := make(map[string]*agent.Repo, len(repos))
	for _, repo := range repos {
		if repo.RepoId == "" || !repo.Enabled {
			continue
		}
		if _, exists := repoByID[repo.RepoId]; !exists {
			repoByID[repo.RepoId] = repo
		}
	}

	unique := make([]*agent.Repo, 0, len(repoByID))
	for _, repo := range repoByID {
		unique = append(unique, repo)
	}

	sort.Slice(unique, func(i, j int) bool {
		return unique[i].RepoId < unique[j].RepoId
	})

	return unique
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
	repoID     string
	enabled    bool
	repoLabel  string
	baseurl    string
	metalink   string
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

func parseAptListFile(contents, source string) []*agent.Repo {
	var repos []*agent.Repo
	scanner := bufio.NewScanner(strings.NewReader(contents))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "deb-src ") {
			continue
		}
		if !strings.HasPrefix(line, "deb ") {
			continue
		}

		uri, suite, components, ok := parseAptListLine(line)
		if !ok {
			continue
		}

		repoID := source + ":" + strconv.Itoa(lineNo)
		repoLabel := suite
		if len(components) > 0 {
			repoLabel = suite + " " + strings.Join(components, " ")
		}
		repos = append(repos, &agent.Repo{
			RepoId:    repoID,
			Enabled:   true,
			RepoLabel: repoLabel,
			Baseurl:   uri,
		})
	}
	return repos
}

func parseAptListLine(line string) (uri string, suite string, components []string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "deb ") {
		return "", "", nil, false
	}

	remainder := strings.TrimSpace(strings.TrimPrefix(trimmed, "deb "))
	if strings.HasPrefix(remainder, "[") {
		closing := strings.Index(remainder, "]")
		if closing < 0 || closing+1 >= len(remainder) {
			return "", "", nil, false
		}
		remainder = strings.TrimSpace(remainder[closing+1:])
	}

	fields := strings.Fields(remainder)
	if len(fields) < 2 {
		return "", "", nil, false
	}
	return fields[0], fields[1], fields[2:], true
}

func parseAptSourcesFile(contents, source string) []*agent.Repo {
	var repos []*agent.Repo
	var block map[string]string
	lineNo := 0
	blockStartLine := 0

	flush := func() {
		if block == nil {
			return
		}
		if !isAptSourcesBlockEnabled(block) {
			block = nil
			return
		}
		types := strings.Fields(strings.ToLower(block["types"]))
		if len(types) == 0 || !containsToken(types, "deb") {
			block = nil
			return
		}

		uris := strings.Fields(block["uris"])
		suites := strings.Fields(block["suites"])
		components := strings.Fields(block["components"])
		if len(uris) == 0 || len(suites) == 0 {
			block = nil
			return
		}

		repoIndex := 0
		for _, uri := range uris {
			for _, suite := range suites {
				repoID := fmt.Sprintf("%s:%d:%d", source, blockStartLine, repoIndex)
				repoIndex++
				repoLabel := suite
				if len(components) > 0 {
					repoLabel = suite + " " + strings.Join(components, " ")
				}
				repos = append(repos, &agent.Repo{
					RepoId:    repoID,
					Enabled:   true,
					RepoLabel: repoLabel,
					Baseurl:   uri,
				})
			}
		}

		block = nil
	}

	scanner := bufio.NewScanner(strings.NewReader(contents))
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if block == nil {
			block = make(map[string]string)
			blockStartLine = lineNo
		}

		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		value := strings.TrimSpace(line[idx+1:])
		block[key] = value
	}
	flush()

	return repos
}

func isAptSourcesBlockEnabled(values map[string]string) bool {
	value, exists := values["enabled"]
	if !exists {
		return true
	}
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized == "1" || normalized == "yes" || normalized == "true"
}

func containsToken(tokens []string, needle string) bool {
	for _, token := range tokens {
		if token == needle {
			return true
		}
	}
	return false
}

func mustReadFile(fs afero.Fs, path string) []byte {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		return nil
	}
	return data
}
