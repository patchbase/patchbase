package matchers

import (
	"strings"

	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/sql"
)

func resolveProductStreams(host sql.Host, repos []*agentpb.Repo, streams []sql.ProductStream) []sql.ProductStream {
	resolved := make([]sql.ProductStream, 0)
	repoMatchFields := enabledRepoMatchFields(repos)
	seen := map[string]struct{}{}

	for _, stream := range streams {
		if !matchesHostDistro(host, stream) {
			continue
		}

		if !matchesStreamArchitecture(host.OsFamily, host.Architecture, stream.Architecture.UnwrapOr("")) {
			continue
		}

		if stream.RepoFamily != "" && stream.RepoFamily != "all" && !matchesRepoFamily(repoMatchFields, stream.RepoFamily) {
			continue
		}

		if _, exists := seen[stream.ID]; exists {
			continue
		}

		seen[stream.ID] = struct{}{}
		resolved = append(resolved, stream)
	}

	return resolved
}

func enabledRepoMatchFields(repos []*agentpb.Repo) []string {
	fields := make([]string, 0, len(repos)*3)
	for _, repo := range repos {
		if repo.Enabled {
			if repo.RepoId != "" {
				fields = append(fields, strings.ToLower(repo.RepoId))
			}
			if repo.RepoLabel != "" {
				fields = append(fields, strings.ToLower(repo.RepoLabel))
			}
			if repo.Baseurl != "" {
				fields = append(fields, strings.ToLower(repo.Baseurl))
			}
		}
	}

	return fields
}

func matchesHostDistro(host sql.Host, stream sql.ProductStream) bool {
	if host.OsMajor != stream.MajorVersion {
		return false
	}

	hostName := strings.ToLower(host.OsName)
	hostFamily := strings.ToLower(host.OsFamily)

	switch stream.Vendor {
	case "rocky":
		return strings.Contains(hostName, "rocky") || strings.Contains(hostFamily, "rocky")
	case "alma":
		return strings.Contains(hostName, "alma") || strings.Contains(hostFamily, "alma")
	case "redhat":
		if strings.Contains(hostName, "rocky") || strings.Contains(hostName, "alma") {
			return false
		}

		if strings.Contains(hostName, "red hat") || strings.Contains(hostName, "rhel") {
			return true
		}

		return hostName == "" && strings.Contains(hostFamily, "rhel")
	case "ubuntu":
		return strings.Contains(hostName, "ubuntu") || strings.Contains(hostFamily, "ubuntu")
	case "debian":
		return strings.Contains(hostName, "debian") || strings.Contains(hostFamily, "debian")
	default:
		return false
	}
}

func matchesRepoFamily(matchFields []string, repoFamily string) bool {
	repoFamily = strings.ToLower(strings.TrimSpace(repoFamily))
	for _, field := range matchFields {
		if strings.Contains(field, repoFamily) {
			return true
		}
	}

	return false
}

func matchesStreamArchitecture(osFamily string, hostArch string, streamArch string) bool {
	streamArch = strings.ToLower(strings.TrimSpace(streamArch))
	if streamArch == "" {
		return true
	}

	// Advisory DB apt streams can represent source-level applicability.
	if strings.EqualFold(osFamily, "apt") && (streamArch == "source" || streamArch == "binary") {
		return true
	}

	return streamArch == strings.ToLower(strings.TrimSpace(hostArch))
}
