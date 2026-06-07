package entities

import db "go.patchbase.net/server/internal/sql"

type Advisory struct {
	ID           string  `json:"id"`
	SourceSystem string  `json:"source_system"`
	RawSourceID  string  `json:"raw_source_id"`
	SourceUrl    *string `json:"source_url"`
	Vendor       string  `json:"vendor"`
	AdvisoryType string  `json:"advisory_type"`
	Severity     *string `json:"severity"`
	Summary      *string `json:"summary"`
	Description  *string `json:"description"`
	PublishedAt  *string `json:"published_at"`
	UpdatedAt    *string `json:"updated_at"`
	EvidenceTier string  `json:"evidence_tier"`
	IsSecurity   bool    `json:"is_security"`
}

func MapAdvisory(adv db.Advisory) Advisory {
	return Advisory{
		ID:           adv.ID,
		SourceSystem: adv.SourceSystem,
		RawSourceID:  adv.RawSourceID,
		SourceUrl:    adv.SourceUrl.Ptr(),
		Vendor:       adv.Vendor,
		AdvisoryType: adv.AdvisoryType,
		Severity:     adv.Severity.Ptr(),
		Summary:      adv.Summary.Ptr(),
		Description:  adv.Description.Ptr(),
		PublishedAt:  adv.PublishedAt.Ptr(),
		UpdatedAt:    adv.UpdatedAt.Ptr(),
		EvidenceTier: adv.EvidenceTier,
		IsSecurity:   adv.IsSecurity,
	}
}
