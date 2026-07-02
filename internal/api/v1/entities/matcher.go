package entities

import "go.patchbase.net/server/internal/utils"

type CVEInfo struct {
	ID    string                `json:"id"`
	URL   string                `json:"url"`
	Score utils.Option[float64] `json:"score"`
}

type DecisionItem struct {
	AdvisoryID           string    `json:"advisory_id"`
	Title                string    `json:"title"`
	FamilyLabel          string    `json:"family_label"`
	PackageName          string    `json:"package_name"`
	InstalledNevra       string    `json:"installed_nevra"`
	FixedNevra           string    `json:"fixed_nevra"`
	PackageStateLabel    string    `json:"package_state_label"`
	PackageStateTone     string    `json:"package_state_tone"`
	PackageStateIcon     string    `json:"package_state_icon"`
	SeverityLabel        string    `json:"severity_label"`
	SeverityTone         string    `json:"severity_tone"`
	StatusLabel          string    `json:"status_label"`
	ActionLabel          string    `json:"action_label"`
	ActionTone           string    `json:"action_tone"`
	EvidenceTier         string    `json:"evidence_tier"`
	ReasonText           string    `json:"reason_text"`
	ComputedAt           string    `json:"computed_at"`
	AdvisorySourceSystem string    `json:"advisory_source_system"`
	AdvisoryURL          string    `json:"advisory_url"`
	AdvisoryUpdatedAt    string    `json:"advisory_updated_at"`
	CVEs                 []CVEInfo `json:"cves"`
}

type DecisionGroup struct {
	FamilyLabel     string                  `json:"family_label"`
	SeverityLabel   string                  `json:"severity_label"`
	SeverityTone    string                  `json:"severity_tone"`
	ActionLabel     string                  `json:"action_label"`
	ActionTone      string                  `json:"action_tone"`
	LatestUpdatedAt string                  `json:"latest_updated_at"`
	AdvisoryCount   int                     `json:"advisory_count"`
	PackageCount    int                     `json:"package_count"`
	Advisories      []DecisionAdvisoryGroup `json:"advisories"`
}

type DecisionAdvisoryGroup struct {
	AdvisoryID           string         `json:"advisory_id"`
	Title                string         `json:"title"`
	SeverityLabel        string         `json:"severity_label"`
	SeverityTone         string         `json:"severity_tone"`
	ActionLabel          string         `json:"action_label"`
	ActionTone           string         `json:"action_tone"`
	EvidenceTier         string         `json:"evidence_tier"`
	ComputedAt           string         `json:"computed_at"`
	AdvisorySourceSystem string         `json:"advisory_source_system"`
	AdvisoryURL          string         `json:"advisory_url"`
	AdvisoryUpdatedAt    string         `json:"advisory_updated_at"`
	CVEs                 []CVEInfo      `json:"cves"`
	PackageCount         int            `json:"package_count"`
	Items                []DecisionItem `json:"items"`
}
