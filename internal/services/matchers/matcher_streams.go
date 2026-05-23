package matchers

import (
	"strings"

	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/sql"
)

func resolveProductStreams(host sql.Host, repos []*agentpb.Repo, streams []sql.ProductStream) []sql.ProductStream {
	resolved := make([]sql.ProductStream, 0)
	enabledRepoIDs := enabledRepoIDs(repos)
	seen := map[string]struct{}{}

	for _, stream := range streams {
		if !matchesHostDistro(host, stream) {
			continue
		}

		if stream.Architecture.IsPresent() && stream.Architecture.UnwrapOr("") != "" && stream.Architecture.UnwrapOr("") != host.Architecture {
			continue
		}

		if stream.RepoFamily != "" && stream.RepoFamily != "all" && !matchesRepoFamily(enabledRepoIDs, stream.RepoFamily) {
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

func enabledRepoIDs(repos []*agentpb.Repo) []string {
	enabled := make([]string, 0, len(repos))
	for _, repo := range repos {
		if repo.Enabled {
			enabled = append(enabled, strings.ToLower(repo.RepoId))
		}
	}

	return enabled
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
	default:
		return false
	}
}

func matchesRepoFamily(enabledRepoIDs []string, repoFamily string) bool {
	repoFamily = strings.ToLower(strings.TrimSpace(repoFamily))
	for _, repoID := range enabledRepoIDs {
		if strings.Contains(repoID, repoFamily) {
			return true
		}
	}

	return false
}
