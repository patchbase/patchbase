// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package entities

import (
	"go.patchbase.net/server/internal/lifecycle"
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/utils"
)

// LifecycleStatus is the API representation of a host's lifecycle status.
// The State field is a stable identifier shared with the dashboard; the
// dashboard renders its own display labels from it.
type LifecycleStatus struct {
	State                string            `json:"state"`
	ProductKey           string            `json:"product_key,omitempty"`
	ProductDisplayName   string            `json:"product_display_name,omitempty"`
	Cycle                string            `json:"cycle,omitempty"`
	StandardSupportEnd   string            `json:"standard_support_end,omitempty"`
	EOL                  string            `json:"eol,omitempty"`
	ExtendedCoverageEnd  string            `json:"extended_coverage_end,omitempty"`
	ExtendedCoverageNote string            `json:"extended_coverage_note,omitempty"`
	SourceURL            string            `json:"source_url,omitempty"`
	DaysRemaining        utils.Option[int] `json:"days_remaining,omitempty"`
}

func NewLifecycleStatus(status lifecycle.Status) LifecycleStatus {
	return LifecycleStatus{
		State:                status.State,
		ProductKey:           status.ProductKey,
		ProductDisplayName:   status.ProductDisplayName,
		Cycle:                status.Cycle,
		StandardSupportEnd:   status.StandardSupportEnd,
		EOL:                  status.EOL,
		ExtendedCoverageEnd:  status.ExtendedCoverageEnd,
		ExtendedCoverageNote: status.ExtendedCoverageNote,
		SourceURL:            status.SourceURL,
		DaysRemaining:        status.DaysRemaining,
	}
}

// NewHostsWithLifecycle maps a slice of [services.HostLifecycleInfo] into
// host entities, attaching the resolved lifecycle status to each host.
func NewHostsWithLifecycle(values []services.HostLifecycleInfo) []Host {
	result := make([]Host, 0, len(values))
	for _, value := range values {
		result = append(result, NewHost(value.HostInfo, WithLifecycle(NewLifecycleStatus(value.Lifecycle))))
	}
	return result
}

// LifecycleCatalogSource is the API representation of catalog provenance.
type LifecycleCatalogSource struct {
	GeneratedAt string   `json:"generated_at"`
	SourceURLs  []string `json:"source_urls"`
}

func NewLifecycleCatalogSource(source lifecycle.CatalogSource) LifecycleCatalogSource {
	return LifecycleCatalogSource{
		GeneratedAt: source.GeneratedAt,
		SourceURLs:  utils.Map(source.SourceURLs, func(s string) string { return s }),
	}
}
