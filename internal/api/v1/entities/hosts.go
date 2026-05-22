package entities

import (
	"time"

	"go.patchbase.net/server/internal/services"
)

const apiTimeLayout = "2006-01-02T15:04:05.000000Z"

type Host struct {
	ID                  string  `json:"id"`
	OnboardingMode      string  `json:"onboarding_mode"`
	ApprovalStatus      string  `json:"approval_status"`
	DisplayName         string  `json:"display_name"`
	Hostname            string  `json:"hostname"`
	IPAddress           string  `json:"ip_address"`
	OSFamily            string  `json:"os_family"`
	OSName              string  `json:"os_name"`
	OSMajor             int32   `json:"os_major"`
	OSVersion           string  `json:"os_version"`
	Architecture        string  `json:"architecture"`
	Status              string  `json:"status"`
	OverallAction       string  `json:"overall_action"`
	CriticalCount       int32   `json:"critical_count"`
	ImportantCount      int32   `json:"important_count"`
	ModerateCount       int32   `json:"moderate_count"`
	ActionableCount     int32   `json:"actionable_count"`
	AvailableUpdates    int32   `json:"available_updates"`
	NeedsReboot         int32   `json:"needs_reboot"`
	NeedsRestart        int32   `json:"needs_restart"`
	NoFix               int32   `json:"no_fix"`
	Unknown             int32   `json:"unknown"`
	LastSeenAt          *string `json:"last_seen_at"`
	LastAdvisoryCheckAt *string `json:"last_advisory_check_at"`
	StateUpdatedAt      *string `json:"state_updated_at"`
	PullLastRunAt       *string `json:"pull_last_run_at"`
	PullLastRunStatus   string  `json:"pull_last_run_status"`
	PullLastRunError    string  `json:"pull_last_run_error"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

type HostSnapshot struct {
	ID                 string  `json:"id"`
	HostID             string  `json:"host_id"`
	CollectedAt        string  `json:"collected_at"`
	ReceivedAt         string  `json:"received_at"`
	RunningKernelNevra string  `json:"running_kernel_nevra"`
	BootTime           *string `json:"boot_time"`
	HasProcessData     bool    `json:"has_process_data"`
}

func NewHost(value services.HostInfo) Host {
	return Host{
		ID:                  value.ID,
		OnboardingMode:      value.OnboardingMode,
		ApprovalStatus:      value.ApprovalStatus,
		DisplayName:         value.DisplayName,
		Hostname:            value.Hostname,
		IPAddress:           value.IPAddress,
		OSFamily:            value.OSFamily,
		OSName:              value.OSName,
		OSMajor:             value.OSMajor,
		OSVersion:           value.OSVersion,
		Architecture:        value.Architecture,
		Status:              value.Status,
		OverallAction:       value.OverallAction,
		CriticalCount:       value.CriticalCount,
		ImportantCount:      value.ImportantCount,
		ModerateCount:       value.ModerateCount,
		ActionableCount:     value.ActionableCount,
		AvailableUpdates:    value.AvailableUpdates,
		NeedsReboot:         value.NeedsReboot,
		NeedsRestart:        value.NeedsRestart,
		NoFix:               value.NoFix,
		Unknown:             value.Unknown,
		LastSeenAt:          formatTimePtr(value.LastSeenAt),
		LastAdvisoryCheckAt: formatTimePtr(value.LastAdvisoryCheckAt),
		StateUpdatedAt:      formatTimePtr(value.StateUpdatedAt),
		PullLastRunAt:       formatTimePtr(value.PullLastRunAt),
		PullLastRunStatus:   value.PullLastRunStatus,
		PullLastRunError:    value.PullLastRunError,
		CreatedAt:           value.CreatedAt.UTC().Format(apiTimeLayout),
		UpdatedAt:           value.UpdatedAt.UTC().Format(apiTimeLayout),
	}
}

func NewHosts(values []services.HostInfo) []Host {
	result := make([]Host, 0, len(values))
	for _, value := range values {
		result = append(result, NewHost(value))
	}
	return result
}

func NewHostSnapshot(value services.HostSnapshotInfo) HostSnapshot {
	return HostSnapshot{
		ID:                 value.ID,
		HostID:             value.HostID,
		CollectedAt:        value.CollectedAt.UTC().Format(apiTimeLayout),
		ReceivedAt:         value.ReceivedAt.UTC().Format(apiTimeLayout),
		RunningKernelNevra: value.RunningKernelNevra,
		BootTime:           formatTimePtr(value.BootTime),
		HasProcessData:     value.HasProcessData,
	}
}

func formatTimePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(apiTimeLayout)
	return &formatted
}
