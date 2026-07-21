// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package entities

import (
	db "go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/utils"
)

type Advisory struct {
	ID           string               `json:"id"`
	SourceSystem string               `json:"source_system"`
	RawSourceID  string               `json:"raw_source_id"`
	SourceUrl    utils.Option[string] `json:"source_url"`
	Vendor       string               `json:"vendor"`
	AdvisoryType string               `json:"advisory_type"`
	Severity     utils.Option[string] `json:"severity"`
	Summary      utils.Option[string] `json:"summary"`
	Description  utils.Option[string] `json:"description"`
	PublishedAt  utils.Option[string] `json:"published_at"`
	UpdatedAt    utils.Option[string] `json:"updated_at"`
	EvidenceTier string               `json:"evidence_tier"`
	IsSecurity   bool                 `json:"is_security"`
}

func MapAdvisory(adv db.Advisory) Advisory {
	return Advisory{
		ID:           adv.ID,
		SourceSystem: adv.SourceSystem,
		RawSourceID:  adv.RawSourceID,
		SourceUrl:    adv.SourceUrl,
		Vendor:       adv.Vendor,
		AdvisoryType: adv.AdvisoryType,
		Severity:     adv.Severity,
		Summary:      adv.Summary,
		Description:  adv.Description,
		PublishedAt:  adv.PublishedAt,
		UpdatedAt:    adv.UpdatedAt,
		EvidenceTier: adv.EvidenceTier,
		IsSecurity:   adv.IsSecurity,
	}
}
