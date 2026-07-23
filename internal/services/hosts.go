// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package services

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do/v2"
	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/apperr"
	"go.patchbase.net/server/internal/events"
	"go.patchbase.net/server/internal/services/matchers"
	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	"go.patchbase.net/server/internal/utils"
	"google.golang.org/protobuf/proto"
)

const (
	defaultNextCheckInSeconds = int32(21600)
	defaultSSHPullFrequency   = 360
	sshPullHistoryLimit       = int32(10)
)

type SSHPullArgs struct {
	HostID string `json:"host_id"`
}

func (SSHPullArgs) Kind() string {
	return "ssh_pull"
}

type Hosts interface {
	CreateRegistrationToken(ctx context.Context, actor ActorRef, userID string, name string, autoApprove bool) (CreatedRegistrationToken, error)
	ListRegistrationTokens(ctx context.Context) ([]RegistrationTokenInfo, error)
	RevokeRegistrationToken(ctx context.Context, actor ActorRef, tokenID string) error
	RegisterAgentHost(ctx context.Context, input *agentpb.RegisterHostRequest) (*agentpb.RegisterHostResponse, error)
	IngestAgentSnapshot(ctx context.Context, hostAccessToken string, payload *agentpb.AgentSnapshot) (*agentpb.SyncResponse, error)
	ListPendingHosts(ctx context.Context) ([]HostInfo, error)
	ApproveHost(ctx context.Context, actor ActorRef, hostID string) (HostInfo, error)
	UpdateHost(ctx context.Context, actor ActorRef, hostID string, input UpdateHostInput) (HostInfo, error)
	UpdateHostNotes(ctx context.Context, hostID string, notes utils.Option[string]) (HostInfo, error)
	DeleteHost(ctx context.Context, actor ActorRef, hostID string) error
	CreateSSHHost(ctx context.Context, actor ActorRef, input CreateSSHHostInput) (CreateSSHHostResult, error)
	OnboardSSHHost(ctx context.Context, actor ActorRef, hostID string) error
	ListHosts(ctx context.Context) ([]HostInfo, error)
	GetHost(ctx context.Context, hostID string) (HostInfo, error)
	GetLatestSnapshot(ctx context.Context, hostID string) (HostSnapshotInfo, error)
	RunSSHPull(ctx context.Context, actor ActorRef, hostID string) error
	ListSSHPullJobs(ctx context.Context, hostID string) ([]HostSSHPullJobInfo, error)
	GetDashboardOverview(ctx context.Context) (DashboardOverview, error)
	CreateManualHost(ctx context.Context, actor ActorRef, displayName string, hostname string) (HostInfo, error)
	IngestManualReport(ctx context.Context, actor ActorRef, hostID string, reportContent []byte) error
	GetCollectorScript(osFamily string) (string, error)
}

// ActorRef describes the user performing an audited action. The user ID and
// email are kept separate so anonymous flows (e.g. failed logins) can supply
// only the email without a known user ID.
type ActorRef struct {
	UserID    string
	Email     string
	IP        string
	UserAgent string
}

// SystemActorRef returns an ActorRef representing a system-triggered action
// (e.g. an SSH pull scheduled by the periodic job runner). The system actor
// is recorded with a synthetic email so audit consumers can filter on it.
func SystemActorRef() ActorRef {
	return ActorRef{ // nolint: exhaustruct
		UserID: "system",
		Email:  "system@patchbase.local",
	}
}

type DashboardOverview struct {
	TotalHosts         int64            `json:"total_hosts"`
	NeedAttention      int64            `json:"need_attention"`
	RebootQueue        int64            `json:"reboot_queue"`
	UnknownInvestigate int64            `json:"unknown_investigate"`
	TotalAdvisories    int64            `json:"total_advisories"`
	TotalScopes        int64            `json:"total_scopes"`
	RecentAdvisories   []RecentAdvisory `json:"recent_advisories"`
}

type RecentAdvisory struct {
	ID           string  `json:"id"`
	SourceSystem string  `json:"source_system"`
	Vendor       string  `json:"vendor"`
	AdvisoryType string  `json:"advisory_type"`
	Severity     *string `json:"severity"`
	Summary      *string `json:"summary"`
	PublishedAt  *string `json:"published_at"`
}

type hosts struct {
	pool               *pgxpool.Pool
	sql                sql.Querier
	random             utils.RandomStringGenerator
	sshRunner          SSHPullRunner
	crypto             utils.Crypto
	injector           do.Injector
	periodicJobManager PeriodicJobManager
	advisoriesService  AdvisorySyncService
	matcher            matchers.Matcher
	settingsService    Settings
	broker             events.Broker
	audit              AuditLogService
}

type CreatedRegistrationToken struct {
	ID        string
	Name      string
	Token     string
	CreatedAt time.Time
}

type RegistrationTokenInfo struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	CreatedBy   string                  `json:"created_by_user_id"`
	CreatedAt   time.Time               `json:"created_at"`
	RevokedAt   utils.Option[time.Time] `json:"revoked_at"`
	LastUsedAt  utils.Option[time.Time] `json:"last_used_at"`
	AutoApprove bool                    `json:"auto_approve"`
}

type SSHPullConfiguration struct {
	Hostname          string `json:"pull_hostname"`
	SSHUser           string `json:"pull_ssh_user"`
	FrequencyMinutes  int32  `json:"pull_frequency_minutes"`
	Onboarded         bool   `json:"onboarded"`
	UsesUniqueKeyPair bool   `json:"uses_unique_key_pair"`
}

type HostInfo struct {
	ID                  string                             `json:"id"`
	OnboardingMode      string                             `json:"onboarding_mode"`
	ApprovalStatus      string                             `json:"approval_status"`
	DisplayName         string                             `json:"display_name"`
	Hostname            string                             `json:"hostname"`
	IPAddress           string                             `json:"ip_address"`
	OSFamily            string                             `json:"os_family"`
	OSName              string                             `json:"os_name"`
	OSMajor             int32                              `json:"os_major"`
	OSVersion           string                             `json:"os_version"`
	Architecture        string                             `json:"architecture"`
	Status              string                             `json:"status"`
	OverallAction       string                             `json:"overall_action"`
	CriticalCount       int32                              `json:"critical_count"`
	ImportantCount      int32                              `json:"important_count"`
	ModerateCount       int32                              `json:"moderate_count"`
	ActionableCount     int32                              `json:"actionable_count"`
	AvailableUpdates    int32                              `json:"available_updates"`
	NeedsReboot         int32                              `json:"needs_reboot"`
	NeedsRestart        int32                              `json:"needs_restart"`
	NoFix               int32                              `json:"no_fix"`
	Unknown             int32                              `json:"unknown"`
	LastSeenAt          utils.Option[time.Time]            `json:"last_seen_at"`
	LastAdvisoryCheckAt utils.Option[time.Time]            `json:"last_advisory_check_at"`
	StateUpdatedAt      utils.Option[time.Time]            `json:"state_updated_at"`
	PullLastRunAt       utils.Option[time.Time]            `json:"pull_last_run_at"`
	PullLastRunStatus   string                             `json:"pull_last_run_status"`
	PullLastRunError    string                             `json:"pull_last_run_error"`
	Notes               utils.Option[string]               `json:"notes"`
	Configuration       utils.Option[SSHPullConfiguration] `json:"configuration,omitempty"`
	CreatedAt           time.Time                          `json:"created_at"`
	UpdatedAt           time.Time                          `json:"updated_at"`
}

type HostSnapshotInfo struct {
	ID                 string                  `json:"id"`
	HostID             string                  `json:"host_id"`
	CollectedAt        time.Time               `json:"collected_at"`
	ReceivedAt         time.Time               `json:"received_at"`
	RunningKernelNevra string                  `json:"running_kernel_nevra"`
	BootTime           utils.Option[time.Time] `json:"boot_time"`
	HasProcessData     bool                    `json:"has_process_data"`
	Payload            []byte                  `json:"payload"`
}

type HostSSHPullJobInfo struct {
	ID          string                  `json:"id"`
	HostID      string                  `json:"host_id"`
	Status      string                  `json:"status"`
	StartedAt   time.Time               `json:"started_at"`
	CompletedAt utils.Option[time.Time] `json:"completed_at"`
	Error       utils.Option[string]    `json:"error"`
}

type UpdateHostInput struct {
	DisplayName          utils.Option[string]
	PullHostname         utils.Option[string]
	PullSSHUser          utils.Option[string]
	PullFrequencyMinutes utils.Option[int32]
}

type CreateSSHHostInput struct {
	DisplayName      string
	Hostname         string
	SSHUser          string
	FrequencyMinutes int32
	UniqueKeyPair    bool
}

type CreateSSHHostResult struct {
	HostID          string `json:"host_id"`
	PublicKey       string `json:"public_key"`
	ApprovalStatus  string `json:"approval_status"`
	LastRunStatus   string `json:"last_run_status"`
	LastRunError    string `json:"last_run_error"`
	HostAccessToken string `json:"host_access_token,omitempty"`
}

func NewHosts(i do.Injector) (Hosts, error) {
	pool, err := do.Invoke[*pgxpool.Pool](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get *pgxpool.Pool: %w", err)
	}
	queries, err := do.Invoke[sql.Querier](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.Querier: %w", err)
	}
	random, err := do.Invoke[utils.RandomStringGenerator](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get random generator: %w", err)
	}
	crypto, err := do.Invoke[utils.Crypto](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get crypto: %w", err)
	}
	periodicJobManager, err := do.Invoke[PeriodicJobManager](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get periodic job manager: %w", err)
	}

	sshRunner, err := do.Invoke[SSHPullRunner](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get ssh runner: %w", err)
	}

	advisoriesService, err := do.Invoke[AdvisorySyncService](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get advisory sync service: %w", err)
	}

	matcher, err := do.Invoke[matchers.Matcher](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get matcher: %w", err)
	}

	settingsService, err := do.Invoke[Settings](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get settings service: %w", err)
	}

	broker, err := do.Invoke[events.Broker](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get events broker: %w", err)
	}

	audit, err := do.Invoke[AuditLogService](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit log service: %w", err)
	}

	return &hosts{
		pool:               pool,
		sql:                queries,
		random:             random,
		sshRunner:          sshRunner,
		crypto:             crypto,
		injector:           i,
		periodicJobManager: periodicJobManager,
		advisoriesService:  advisoriesService,
		matcher:            matcher,
		settingsService:    settingsService,
		broker:             broker,
		audit:              audit,
	}, nil
}

func (s *hosts) CreateRegistrationToken(ctx context.Context, actor ActorRef, userID string, name string, autoApprove bool) (CreatedRegistrationToken, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		trimmed = "Registration token"
	}

	plain := "pb_reg_" + s.random.Hex(24)
	created, err := s.sql.InsertRegistrationToken(ctx, sql.InsertRegistrationTokenParams{
		ID:              id.New("rtok"),
		Name:            trimmed,
		TokenHash:       utils.SHA256(plain),
		CreatedByUserID: userID,
		AutoApprove:     autoApprove,
	})
	if err != nil {
		return CreatedRegistrationToken{}, fmt.Errorf("insert registration token: %w", err)
	}

	user, err := s.sql.GetUserByID(ctx, userID)
	if err != nil {
		utils.GetLogger(ctx).
			ErrorContext(ctx, "failed to load user for audit context after registration token creation",
				"user_id", userID, "token_id", created.ID, "error", err)
	} else {
		s.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
			ActorID:    user.ID,
			ActorEmail: user.Email,
			Action:     auditLogActionTokenCreate,
			TargetType: auditLogTargetTypeRegistrationToken,
			TargetID:   created.ID,
			Metadata: map[string]any{
				"name":         created.Name,
				"auto_approve": created.AutoApprove,
			},
			IPAddress: actor.IP,
			UserAgent: actor.UserAgent,
		})
	}

	return CreatedRegistrationToken{
		ID:        created.ID,
		Name:      created.Name,
		Token:     plain,
		CreatedAt: created.CreatedAt.Time.UTC(),
	}, nil
}

func (s *hosts) ListRegistrationTokens(ctx context.Context) ([]RegistrationTokenInfo, error) {
	rows, err := s.sql.ListRegistrationTokens(ctx)
	if err != nil {
		return nil, fmt.Errorf("list registration tokens: %w", err)
	}
	items := make([]RegistrationTokenInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, RegistrationTokenInfo{
			ID:          row.ID,
			Name:        row.Name,
			CreatedBy:   row.CreatedByUserID,
			CreatedAt:   row.CreatedAt.Time.UTC(),
			RevokedAt:   sql.NewTimeOption(row.RevokedAt),
			LastUsedAt:  sql.NewTimeOption(row.LastUsedAt),
			AutoApprove: row.AutoApprove,
		})
	}
	return items, nil
}

func (s *hosts) RevokeRegistrationToken(ctx context.Context, actor ActorRef, tokenID string) error {
	revoked, err := sql.Required(s.sql.RevokeRegistrationToken(ctx, tokenID))(apperr.ErrTokenAlreadyRevoked)
	if err != nil {
		return fmt.Errorf("revoke registration token: %w", err)
	}

	s.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
		ActorID:    actor.UserID,
		ActorEmail: actor.Email,
		Action:     auditLogActionTokenRevoke,
		TargetType: auditLogTargetTypeRegistrationToken,
		TargetID:   revoked.ID,
		Metadata: map[string]any{
			"name": revoked.Name,
		},
		IPAddress: actor.IP,
		UserAgent: actor.UserAgent,
	})

	return nil
}

func (s *hosts) RegisterAgentHost(ctx context.Context, input *agentpb.RegisterHostRequest) (*agentpb.RegisterHostResponse, error) {
	registrationToken := strings.TrimSpace(input.RegistrationToken)
	if registrationToken == "" {
		return nil, apperr.ErrInvalidRegistrationToken
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin register agent host transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	queries := sql.New(tx)
	regToken, err := sql.Required(queries.GetActiveRegistrationTokenByHash(ctx, utils.SHA256(registrationToken)))(apperr.ErrInvalidRegistrationToken)
	if err != nil {
		return nil, fmt.Errorf("get active registration token: %w", err)
	}

	hostname := strings.TrimSpace(input.Hostname)
	machineID := strings.TrimSpace(input.MachineId)
	ipAddress := strings.TrimSpace(input.Metadata.IpAddress)
	osName := utils.NonZeroOption(strings.TrimSpace(input.Metadata.OsName)).UnwrapOr("Unknown")
	osVersion := utils.NonZeroOption(strings.TrimSpace(input.Metadata.OsVersion)).UnwrapOr("unknown")
	architecture := normalizeRegistrationArchitecture(strings.TrimSpace(input.Metadata.Architecture))
	if architecture == "" {
		architecture = "unknown"
	}
	displayName := utils.NonZeroOption(hostname)

	insertParams := sql.InsertAgentHostParams{
		ID:           id.New("h"),
		DisplayName:  displayName,
		MachineID:    utils.NonZeroOption(machineID),
		Hostname:     utils.NonZeroOption(hostname),
		IpAddress:    utils.NonZeroOption(ipAddress),
		OsName:       osName,
		OsVersion:    osVersion,
		Architecture: architecture,
	}

	var host sql.Host
	if regToken.AutoApprove {
		host, err = queries.InsertAgentHostApproved(ctx, sql.InsertAgentHostApprovedParams(insertParams))
	} else {
		host, err = queries.InsertAgentHost(ctx, insertParams)
	}
	if err != nil {
		if sql.IsUniqueViolation(err, "hosts_display_name_unique_idx") {
			return nil, apperr.ErrDuplicateHostDisplayName
		}
		return nil, fmt.Errorf("insert agent host: %w", err)
	}

	hostAccessToken := "pb_host_" + s.random.Hex(24)
	_, err = queries.InsertHostAccessToken(ctx, sql.InsertHostAccessTokenParams{
		ID:        id.New("htok"),
		HostID:    host.ID,
		TokenHash: utils.SHA256(hostAccessToken),
	})
	if err != nil {
		return nil, fmt.Errorf("insert host access token: %w", err)
	}

	if err := queries.TouchRegistrationTokenLastUsed(ctx, regToken.ID); err != nil {
		return nil, fmt.Errorf("touch registration token last used: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit register agent host transaction: %w", err)
	}

	s.broker.Publish(events.NewHostsUpdatedEvent())

	if tokenOwner, ownerErr := s.sql.GetUserByID(ctx, regToken.CreatedByUserID); ownerErr == nil {
		s.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
			ActorID:    tokenOwner.ID,
			ActorEmail: tokenOwner.Email,
			Action:     auditLogActionHostCreate,
			TargetType: auditLogTargetTypeHost,
			TargetID:   host.ID,
			Metadata: map[string]any{
				"onboarding_mode":       "agent",
				"display_name":          hostname,
				"machine_id":            machineID,
				"os_name":               osName,
				"os_version":            osVersion,
				"architecture":          architecture,
				"registration_token_id": regToken.ID,
			},
			IPAddress: ipAddress,
		})
	} else {
		utils.GetLogger(ctx).
			ErrorContext(ctx, "failed to load token owner for agent host audit",
				"host_id", host.ID, "token_id", regToken.ID, "user_id", regToken.CreatedByUserID, "error", ownerErr)
	}

	return &agentpb.RegisterHostResponse{
		HostId:          host.ID,
		HostAccessToken: hostAccessToken,
		ApprovalStatus:  host.ApprovalStatus,
	}, nil
}

func (s *hosts) IngestAgentSnapshot(ctx context.Context, hostAccessToken string, snapshot *agentpb.AgentSnapshot) (*agentpb.SyncResponse, error) {
	trimmedToken := strings.TrimSpace(hostAccessToken)
	if trimmedToken == "" {
		return nil, apperr.ErrInvalidHostAccessToken
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin snapshot transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	queries := sql.New(tx)
	tokenRow, err := sql.Required(queries.GetActiveHostAccessTokenByHash(ctx, utils.SHA256(trimmedToken)))(apperr.ErrInvalidHostAccessToken)
	if err != nil {
		return nil, fmt.Errorf("get active host access token: %w", err)
	}

	host, err := sql.Required(queries.GetHostByID(ctx, tokenRow.HostID))(apperr.ErrHostNotFound)
	if err != nil {
		return nil, fmt.Errorf("get host by id: %w", err)
	}
	if host.ApprovalStatus != "approved" {
		return nil, apperr.ErrHostNotApproved
	}

	hostPayload := snapshot.GetHost()
	if hostPayload != nil && host.MachineID.IsPresent() && hostPayload.GetMachineId() != "" && host.MachineID.Unwrap() != hostPayload.GetMachineId() {
		return nil, apperr.ErrHostIdentityMismatch
	}

	collectedAt := time.Now().UTC()
	if snapshot.GetSentAt() != nil {
		collectedAt = snapshot.GetSentAt().AsTime().UTC()
	}

	bootTime := pgtype.Timestamptz{} // nolint: exhaustruct
	if snapshot.GetHost() != nil && snapshot.GetHost().GetBootTime() != nil {
		bootTime = sql.TimestamptzFromTime(snapshot.GetHost().GetBootTime().AsTime().UTC())
	}

	runningKernel := ""
	hasProcessData := false
	if snapshot.GetRuntime() != nil {
		runningKernel = snapshot.GetRuntime().GetKernelRunning()
		hasProcessData = len(snapshot.GetRuntime().GetProcesses()) > 0
	}
	payload, _ := proto.Marshal(snapshot)

	snapshotRow, err := queries.InsertHostSnapshot(ctx, sql.InsertHostSnapshotParams{
		ID:                 id.New("snap"),
		HostID:             host.ID,
		CollectedAt:        sql.TimestamptzFromTime(collectedAt),
		Payload:            payload,
		RunningKernelNevra: runningKernel,
		BootTime:           bootTime,
		HasProcessData:     hasProcessData,
	})
	if err != nil {
		return nil, fmt.Errorf("insert host snapshot: %w", err)
	}

	machineID := ""
	hostname := ""
	ipAddress := ""
	osFamily := "unknown"
	osName := "Unknown"
	osMajor := int32(0)
	osVersion := "unknown"
	architecture := "unknown"
	availableUpdates := int32(0)
	if hostPayload != nil {
		machineID = hostPayload.GetMachineId()
		hostname = hostPayload.GetHostname()
		if len(hostPayload.GetIpAddresses()) > 0 {
			ipAddress = strings.TrimSpace(hostPayload.GetIpAddresses()[0])
		}
		osFamily = normalizeOSFamily(hostPayload.GetOsFamily())
		if hostPayload.GetOsName() != "" {
			osName = hostPayload.GetOsName()
		}
		osMajor = hostPayload.GetOsMajor()
		if hostPayload.GetOsVersion() != "" {
			osVersion = hostPayload.GetOsVersion()
		}
		architecture = normalizeArchitecture(hostPayload.GetArchitecture())
		availableUpdates = hostPayload.GetAvailablePackageUpdateCount()
	}

	_, err = queries.UpdateHostFromSnapshot(ctx, sql.UpdateHostFromSnapshotParams{
		ID:             host.ID,
		MachineID:      utils.NonZeroOption(machineID).FlatMap(utils.EmptySpaceString),
		Hostname:       utils.NonZeroOption(hostname).FlatMap(utils.EmptySpaceString),
		IpAddress:      utils.NonZeroOption(ipAddress).FlatMap(utils.EmptySpaceString),
		OsFamily:       osFamily,
		OsName:         osName,
		OsMajor:        osMajor,
		OsVersion:      osVersion,
		Architecture:   architecture,
		LastSeenAt:     sql.TimestamptzFromTime(collectedAt),
		LastSnapshotID: utils.NonZeroOption(snapshotRow.ID).FlatMap(utils.EmptySpaceString),
	})
	if err != nil {
		return nil, fmt.Errorf("update host from snapshot: %w", err)
	}

	overallAction := "none"
	if availableUpdates > 0 {
		overallAction = "update_package"
	}

	if err := queries.UpsertHostCurrentState(ctx, sql.UpsertHostCurrentStateParams{
		HostID:           host.ID,
		SnapshotID:       snapshotRow.ID,
		OverallAction:    overallAction,
		CriticalCount:    0,
		ImportantCount:   0,
		ModerateCount:    0,
		ActionableCount:  availableUpdates,
		AvailableUpdates: availableUpdates,
		NeedsReboot:      0,
		NeedsRestart:     0,
		NoFix:            0,
		Unknown:          0,
	}); err != nil {
		return nil, fmt.Errorf("upsert host current state: %w", err)
	}

	if err := queries.TouchHostAccessTokenLastUsed(ctx, tokenRow.ID); err != nil {
		return nil, fmt.Errorf("touch host access token last used: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit snapshot transaction: %w", err)
	}

	// Resolve and update advisory scope key (post-commit, outside the transaction using s.sql)
	scopeKey, err := s.advisoriesService.ResolveScopeKey(ctx, osFamily, osName, osVersion, osMajor, architecture)
	if err == nil {
		var registerErr error
		if scopeKey != "" {
			registerErr = s.advisoriesService.RegisterScopeDemand(ctx, scopeKey)
			if registerErr != nil {
				utils.GetLogger(ctx).Warn("register scope demand failed", "error", registerErr)
			}
		}
		if registerErr == nil {
			err = s.sql.UpdateHostAdvisoryScopeKey(ctx, sql.UpdateHostAdvisoryScopeKeyParams{
				ID:               host.ID,
				AdvisoryScopeKey: utils.NonZeroOption(scopeKey),
			})
			if err != nil {
				utils.GetLogger(ctx).Warn("update host advisory scope key failed", "error", err)
			}
		}
	} else {
		utils.GetLogger(ctx).Warn("resolve scope key failed", "error", err)
	}

	// Run MatchSnapshot post-commit
	if _, err := s.matcher.MatchSnapshot(ctx, host.ID, snapshotRow.ID); err != nil {
		utils.GetLogger(ctx).Warn("matching snapshot failed", "host_id", host.ID, "snapshot_id", snapshotRow.ID, "error", err)
	}

	s.broker.Publish(events.NewHostsUpdatedEvent())
	s.broker.Publish(events.NewHostSnapshotEvent(host.ID))

	return &agentpb.SyncResponse{
		Accepted:           true,
		HostId:             host.ID,
		SnapshotId:         snapshotRow.ID,
		NextCheckInSeconds: defaultNextCheckInSeconds,
	}, nil
}

func (s *hosts) ListPendingHosts(ctx context.Context) ([]HostInfo, error) {
	rows, err := s.sql.ListPendingHosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pending hosts: %w", err)
	}
	items := make([]HostInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapHost(row, nil))
	}
	return items, nil
}

func (s *hosts) ApproveHost(ctx context.Context, actor ActorRef, hostID string) (HostInfo, error) {
	host, err := sql.Required(s.sql.ApproveHostByID(ctx, hostID))(apperr.ErrHostNotFound)
	if err != nil {
		return HostInfo{}, fmt.Errorf("approve host: %w", err)
	}
	s.broker.Publish(events.NewHostsUpdatedEvent())
	s.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
		ActorID:    actor.UserID,
		ActorEmail: actor.Email,
		Action:     auditLogActionHostApprove,
		TargetType: auditLogTargetTypeHost,
		TargetID:   host.ID,
		IPAddress:  actor.IP,
	})
	return mapHost(host, nil), nil
}

func (s *hosts) UpdateHost(ctx context.Context, actor ActorRef, hostID string, input UpdateHostInput) (HostInfo, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return HostInfo{}, fmt.Errorf("begin update host transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	queries := sql.New(tx)
	host, err := sql.Required(queries.GetHostByID(ctx, hostID))(apperr.ErrHostNotFound)
	if err != nil {
		return HostInfo{}, fmt.Errorf("get host by id: %w", err)
	}
	if err := queries.LockHost(ctx, hostID); err != nil {
		return HostInfo{}, fmt.Errorf("lock host: %w", err)
	}

	hasSSHFields := input.PullHostname.IsPresent() || input.PullSSHUser.IsPresent() || input.PullFrequencyMinutes.IsPresent()
	if hasSSHFields && host.OnboardingMode != "ssh" {
		return HostInfo{}, apperr.ErrSSHFieldsForNonSSH
	}

	metadata := make(map[string]any)
	if displayName, ok := input.DisplayName.Get(); ok {
		_, err = queries.UpdateHostDisplayName(ctx, sql.UpdateHostDisplayNameParams{ID: hostID, DisplayName: utils.Some(displayName)})
		if err != nil {
			if sql.IsUniqueViolation(err, "hosts_display_name_unique_idx") {
				return HostInfo{}, apperr.ErrDuplicateHostDisplayName
			}
			return HostInfo{}, fmt.Errorf("update host display name: %w", err)
		}
		metadata["display_name"] = map[string]any{
			"from": host.DisplayName.UnwrapOr(""),
			"to":   displayName,
		}
	}

	oldFrequency := int32(defaultSSHPullFrequency)
	shouldReschedule := false
	if hasSSHFields {
		config, configErr := queries.GetSSHPullConfigByHostID(ctx, hostID)
		if configErr != nil {
			return HostInfo{}, fmt.Errorf("get ssh pull config: %w", configErr)
		}
		if config.PullFrequencyMinutes != nil {
			oldFrequency = *config.PullFrequencyMinutes
		}
		shouldReschedule = input.PullFrequencyMinutes.IsPresent() && config.Onboarded
		if input.PullHostname.IsPresent() {
			metadata["pull_hostname"] = map[string]any{
				"from": config.PullHostname,
				"to":   input.PullHostname.Unwrap(),
			}
		}
		if input.PullSSHUser.IsPresent() {
			metadata["pull_ssh_user"] = map[string]any{
				"from": config.PullSshUser.UnwrapOr(""),
				"to":   input.PullSSHUser.Unwrap(),
			}
		}
		var frequency *int32
		if input.PullFrequencyMinutes.IsPresent() {
			value := input.PullFrequencyMinutes.Unwrap()
			if value < 5 {
				return HostInfo{}, apperr.ErrInvalidFrequency
			}
			frequency = &value
			metadata["pull_frequency_minutes"] = map[string]any{"from": config.PullFrequencyMinutes, "to": value}
		}
		_, err = queries.UpdateSSHPullConfig(ctx, sql.UpdateSSHPullConfigParams{
			PullHostname:         input.PullHostname,
			PullSshUser:          input.PullSSHUser,
			PullFrequencyMinutes: frequency,
			HostID:               hostID,
		})
		if err != nil {
			if sql.IsUniqueViolation(err, "host_ssh_pull_pull_hostname_unique_idx") {
				return HostInfo{}, apperr.ErrDuplicateSSHPullHostname
			}
			return HostInfo{}, fmt.Errorf("update ssh pull config: %w", err)
		}
	}

	if shouldReschedule {
		newFrequency := input.PullFrequencyMinutes.Unwrap()
		if err := s.periodicJobManager.AddSSHPullJob(ctx, hostID, newFrequency, false); err != nil {
			if restoreErr := s.periodicJobManager.AddSSHPullJob(ctx, hostID, oldFrequency, false); restoreErr != nil {
				return HostInfo{}, fmt.Errorf("reschedule ssh pull job: %w; restore previous schedule: %v", err, restoreErr)
			}
			return HostInfo{}, fmt.Errorf("reschedule ssh pull job: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		if shouldReschedule {
			if restoreErr := s.periodicJobManager.AddSSHPullJob(ctx, hostID, oldFrequency, false); restoreErr != nil {
				return HostInfo{}, fmt.Errorf("commit update host transaction: %w; restore previous schedule: %v", err, restoreErr)
			}
		}
		return HostInfo{}, fmt.Errorf("commit update host transaction: %w", err)
	}

	updated, err := s.GetHost(ctx, hostID)
	if err != nil {
		return HostInfo{}, err
	}
	s.broker.Publish(events.NewHostsUpdatedEvent())
	s.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
		ActorID:    actor.UserID,
		ActorEmail: actor.Email,
		Action:     auditLogActionHostUpdate,
		TargetType: auditLogTargetTypeHost,
		TargetID:   hostID,
		Metadata:   metadata,
		IPAddress:  actor.IP,
		UserAgent:  actor.UserAgent,
	})
	return updated, nil
}

func (s *hosts) UpdateHostNotes(ctx context.Context, hostID string, notes utils.Option[string]) (HostInfo, error) {
	normalized := utils.None[string]()
	if notes.IsPresent() {
		value := strings.TrimRightFunc(notes.Unwrap(), unicode.IsSpace)
		if utf8.RuneCountInString(value) > 8192 {
			return HostInfo{}, apperr.ErrNotesTooLarge
		}
		if value != "" {
			normalized = utils.Some(value)
		}
	}

	_, err := sql.Required(s.sql.UpdateHostNotesByID(ctx, sql.UpdateHostNotesByIDParams{
		ID: hostID, Notes: normalized,
	}))(apperr.ErrHostNotFound)
	if err != nil {
		return HostInfo{}, fmt.Errorf("update host notes: %w", err)
	}

	updated, err := s.GetHost(ctx, hostID)
	if err != nil {
		return HostInfo{}, err
	}
	s.broker.Publish(events.NewHostsUpdatedEvent())
	return updated, nil
}

func (s *hosts) CreateSSHHost(ctx context.Context, actor ActorRef, input CreateSSHHostInput) (CreateSSHHostResult, error) {
	frequency := input.FrequencyMinutes
	if frequency <= 0 {
		frequency = defaultSSHPullFrequency
	}

	var responsePublicKey string
	var dbPublicKey, dbPrivateKey utils.Option[string]

	if input.UniqueKeyPair {
		publicKey, privateKey, err := utils.GenerateSSHKeyPair()
		if err != nil {
			return CreateSSHHostResult{}, fmt.Errorf("generate ssh key pair: %w", err)
		}
		encryptedPrivateKey, err := s.crypto.Encrypt(privateKey)
		if err != nil {
			return CreateSSHHostResult{}, fmt.Errorf("encrypt private key: %w", err)
		}
		dbPublicKey = utils.Some(publicKey)
		dbPrivateKey = utils.Some(encryptedPrivateKey)
		responsePublicKey = publicKey
	} else {
		globalKey, err := s.settingsService.GetGlobalSSHKeyPair(ctx)
		if err != nil {
			return CreateSSHHostResult{}, fmt.Errorf("get global ssh key: %w", err)
		}
		responsePublicKey = globalKey.PublicKey
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CreateSSHHostResult{}, fmt.Errorf("begin create ssh host transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	queries := sql.New(tx)

	host, err := queries.InsertSSHHost(ctx, sql.InsertSSHHostParams{
		ID:                   id.New("h"),
		DisplayName:          utils.Some(input.DisplayName),
		Hostname:             utils.Some(input.Hostname),
		IpAddress:            utils.None[string](),
		PullSshUser:          utils.Some(input.SSHUser),
		PullFrequencyMinutes: &frequency,
		PullPublicKey:        dbPublicKey,
		PullPrivateKey:       dbPrivateKey,
	})
	if err != nil {
		if sql.IsUniqueViolation(err, "hosts_display_name_unique_idx") {
			return CreateSSHHostResult{}, apperr.ErrDuplicateHostDisplayName
		}
		if sql.IsUniqueViolation(err, "host_ssh_pull_pull_hostname_unique_idx") {
			return CreateSSHHostResult{}, apperr.ErrDuplicateSSHPullHostname
		}
		return CreateSSHHostResult{}, fmt.Errorf("insert ssh host: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return CreateSSHHostResult{}, fmt.Errorf("commit create ssh host transaction: %w", err)
	}

	s.broker.Publish(events.NewHostsUpdatedEvent())
	s.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
		ActorID:    actor.UserID,
		ActorEmail: actor.Email,
		Action:     auditLogActionHostCreate,
		TargetType: auditLogTargetTypeHost,
		TargetID:   host.Host.ID,
		Metadata: map[string]any{
			"onboarding_mode": "ssh",
			"display_name":    input.DisplayName,
			"hostname":        input.Hostname,
			"ssh_user":        input.SSHUser,
			"unique_key_pair": input.UniqueKeyPair,
		},
		IPAddress: actor.IP,
		UserAgent: actor.UserAgent,
	})

	return CreateSSHHostResult{
		HostID:          host.Host.ID,
		PublicKey:       responsePublicKey,
		ApprovalStatus:  host.Host.ApprovalStatus,
		LastRunStatus:   "",
		LastRunError:    "",
		HostAccessToken: "",
	}, nil
}

func (s *hosts) OnboardSSHHost(ctx context.Context, actor ActorRef, hostID string) error {
	host, err := sql.Required(s.sql.GetHostByID(ctx, hostID))(apperr.ErrHostNotFound)
	if err != nil {
		return fmt.Errorf("get host: %w", err)
	}

	if host.OnboardingMode != "ssh" {
		return fmt.Errorf("host is not an SSH host")
	}

	cfg, err := s.sql.GetSSHPullConfigByHostID(ctx, hostID)
	if err != nil {
		return fmt.Errorf("get ssh pull config: %w", err)
	}

	frequency := int32(defaultSSHPullFrequency)
	if cfg.PullFrequencyMinutes != nil && *cfg.PullFrequencyMinutes > 0 {
		frequency = *cfg.PullFrequencyMinutes
	}

	if err := s.sql.SetSSHPullOnboarded(ctx, sql.SetSSHPullOnboardedParams{
		HostID:    hostID,
		Onboarded: true,
	}); err != nil {
		return fmt.Errorf("set ssh pull onboarded: %w", err)
	}

	if err := s.periodicJobManager.AddSSHPullJob(ctx, hostID, frequency, true); err != nil {
		return fmt.Errorf("add periodic job: %w", err)
	}

	s.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
		ActorID:    actor.UserID,
		ActorEmail: actor.Email,
		Action:     auditLogActionSSHOnboard,
		TargetType: auditLogTargetTypeHost,
		TargetID:   hostID,
		IPAddress:  actor.IP,
	})

	return nil
}

func (s *hosts) ListHosts(ctx context.Context) ([]HostInfo, error) {
	rows, err := s.sql.ListHostsWithState(ctx)
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}
	return utils.Map(rows, mapHostWithState), nil
}

func (s *hosts) GetHost(ctx context.Context, hostID string) (HostInfo, error) {
	row, err := sql.Required(s.sql.GetHostWithStateByID(ctx, hostID))(apperr.ErrHostNotFound)
	if err != nil {
		return HostInfo{}, fmt.Errorf("get host: %w", err)
	}
	return mapHostWithStateByID(row), nil
}

func (s *hosts) GetLatestSnapshot(ctx context.Context, hostID string) (HostSnapshotInfo, error) {
	row, err := sql.Required(s.sql.GetLatestHostSnapshotByHostID(ctx, hostID))(apperr.ErrSnapshotNotFound)
	if err != nil {
		return HostSnapshotInfo{}, fmt.Errorf("get latest snapshot: %w", err)
	}
	return HostSnapshotInfo{
		ID:                 row.ID,
		HostID:             row.HostID,
		CollectedAt:        row.CollectedAt.Time.UTC(),
		ReceivedAt:         row.ReceivedAt.Time.UTC(),
		RunningKernelNevra: row.RunningKernelNevra,
		BootTime:           sql.NewTimeOption(row.BootTime),
		HasProcessData:     row.HasProcessData,
		Payload:            row.Payload,
	}, nil
}

func (s *hosts) DeleteHost(ctx context.Context, actor ActorRef, hostID string) error {
	trimmedHostID := strings.TrimSpace(hostID)
	if trimmedHostID == "" {
		return apperr.ErrHostNotFound
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin delete host transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	queries := sql.New(tx)
	_, err = sql.Required(queries.GetHostByID(ctx, trimmedHostID))(apperr.ErrHostNotFound)
	if err != nil {
		return fmt.Errorf("get host by id: %w", err)
	}

	if err := queries.ClearHostLastSnapshotByID(ctx, trimmedHostID); err != nil {
		return fmt.Errorf("clear host last snapshot: %w", err)
	}
	if err := queries.DeleteHostCurrentStateByHostID(ctx, trimmedHostID); err != nil {
		return fmt.Errorf("delete host current state: %w", err)
	}
	if err := queries.DeleteHostAccessTokensByHostID(ctx, trimmedHostID); err != nil {
		return fmt.Errorf("delete host access tokens: %w", err)
	}
	if err := queries.DeleteHostSnapshotsByHostID(ctx, trimmedHostID); err != nil {
		return fmt.Errorf("delete host snapshots: %w", err)
	}
	_, err = sql.Required(queries.DeleteHostByID(ctx, trimmedHostID))(apperr.ErrHostNotFound)
	if err != nil {
		return fmt.Errorf("delete host: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete host transaction: %w", err)
	}

	if err := s.periodicJobManager.RemoveSSHPullJob(ctx, trimmedHostID); err != nil {
		utils.GetLogger(ctx).
			ErrorContext(ctx, "failed to remove periodic SSH pull job after host deletion", "host_id", trimmedHostID, "error", err)
	}

	s.broker.Publish(events.NewHostsUpdatedEvent())
	s.broker.Publish(events.NewHostDeletedEvent(trimmedHostID))
	s.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
		ActorID:    actor.UserID,
		ActorEmail: actor.Email,
		Action:     auditLogActionHostDelete,
		TargetType: auditLogTargetTypeHost,
		TargetID:   trimmedHostID,
		IPAddress:  actor.IP,
	})

	return nil
}

func normalizeOSFamily(value agentpb.OsFamily) string {
	switch value {
	case agentpb.OsFamily_OS_FAMILY_RPM:
		return "rpm"
	case agentpb.OsFamily_OS_FAMILY_APT:
		return "apt"
	case agentpb.OsFamily_OS_FAMILY_UNSPECIFIED:
		return "unknown"
	default:
		return "unknown"
	}
}

func normalizeArchitecture(value agentpb.Architecture) string {
	switch value {
	case agentpb.Architecture_ARCHITECTURE_X86_64:
		return "x86_64"
	case agentpb.Architecture_ARCHITECTURE_AARCH64:
		return "aarch64"
	case agentpb.Architecture_ARCHITECTURE_RISCV64:
		return "riscv64"
	case agentpb.Architecture_ARCHITECTURE_UNSPECIFIED:
		return "unknown"
	default:
		return "unknown"
	}
}

func normalizeRegistrationArchitecture(value string) string {
	switch strings.ToLower(value) {
	case "amd64", "x86_64":
		return "x86_64"
	case "arm64", "aarch64":
		return "aarch64"
	default:
		return value
	}
}

func mapHost(host sql.Host, state *sql.HostCurrentState) HostInfo {
	var overallAction = "none"
	var criticalCount, importantCount, moderateCount, actionableCount, availableUpdates, needsReboot, needsRestart, noFix, unknown int32
	var stateUpdatedAt = utils.None[time.Time]()
	if state != nil {
		overallAction = state.OverallAction
		criticalCount = state.CriticalCount
		importantCount = state.ImportantCount
		moderateCount = state.ModerateCount
		actionableCount = state.ActionableCount
		availableUpdates = state.AvailableUpdates
		needsReboot = state.NeedsReboot
		needsRestart = state.NeedsRestart
		noFix = state.NoFix
		unknown = state.Unknown
		stateUpdatedAt = sql.NewTimeOption(state.UpdatedAt)
	}

	createdAt := host.CreatedAt.Time.UTC()
	updatedAt := host.UpdatedAt.Time.UTC()
	return HostInfo{
		ID:                  host.ID,
		OnboardingMode:      host.OnboardingMode,
		ApprovalStatus:      host.ApprovalStatus,
		DisplayName:         host.DisplayName.UnwrapOr(""),
		Hostname:            host.Hostname.UnwrapOr(""),
		IPAddress:           host.IpAddress.UnwrapOr(""),
		OSFamily:            host.OsFamily,
		OSName:              host.OsName,
		OSMajor:             host.OsMajor,
		OSVersion:           host.OsVersion,
		Architecture:        host.Architecture,
		Status:              host.Status,
		OverallAction:       overallAction,
		CriticalCount:       criticalCount,
		ImportantCount:      importantCount,
		ModerateCount:       moderateCount,
		ActionableCount:     actionableCount,
		AvailableUpdates:    availableUpdates,
		NeedsReboot:         needsReboot,
		NeedsRestart:        needsRestart,
		NoFix:               noFix,
		Unknown:             unknown,
		LastSeenAt:          sql.NewTimeOption(host.LastSeenAt),
		LastAdvisoryCheckAt: sql.NewTimeOption(host.LastAdvisoryCheckAt),
		StateUpdatedAt:      stateUpdatedAt,
		PullLastRunAt:       utils.None[time.Time](),
		PullLastRunStatus:   "",
		PullLastRunError:    "",
		Notes:               host.Notes,
		Configuration:       utils.None[SSHPullConfiguration](),
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}
}

func mapHostWithState(row sql.ListHostsWithStateRow) HostInfo {
	createdAt := row.Host.CreatedAt.Time.UTC()
	updatedAt := row.Host.UpdatedAt.Time.UTC()
	configuration := utils.None[SSHPullConfiguration]()
	if row.Host.OnboardingMode == "ssh" {
		frequency := int32(defaultSSHPullFrequency)
		if row.PullFrequencyMinutes != nil {
			frequency = *row.PullFrequencyMinutes
		}
		configuration = utils.Some(SSHPullConfiguration{
			Hostname:          row.PullHostname.UnwrapOr(""),
			SSHUser:           row.PullSshUser.UnwrapOr(""),
			FrequencyMinutes:  frequency,
			Onboarded:         row.Onboarded != nil && *row.Onboarded,
			UsesUniqueKeyPair: row.UsesUniqueKeyPair,
		})
	}
	return HostInfo{
		ID:                  row.Host.ID,
		OnboardingMode:      row.Host.OnboardingMode,
		ApprovalStatus:      row.Host.ApprovalStatus,
		DisplayName:         row.Host.DisplayName.UnwrapOr(""),
		Hostname:            row.Host.Hostname.UnwrapOr(""),
		IPAddress:           row.Host.IpAddress.UnwrapOr(""),
		OSFamily:            row.Host.OsFamily,
		OSName:              row.Host.OsName,
		OSMajor:             row.Host.OsMajor,
		OSVersion:           row.Host.OsVersion,
		Architecture:        row.Host.Architecture,
		Status:              row.Host.Status,
		OverallAction:       row.OverallAction,
		CriticalCount:       row.CriticalCount,
		ImportantCount:      row.ImportantCount,
		ModerateCount:       row.ModerateCount,
		ActionableCount:     row.ActionableCount,
		AvailableUpdates:    row.AvailableUpdates,
		NeedsReboot:         row.NeedsReboot,
		NeedsRestart:        row.NeedsRestart,
		NoFix:               row.NoFix,
		Unknown:             row.Unknown,
		LastSeenAt:          sql.NewTimeOption(row.Host.LastSeenAt),
		LastAdvisoryCheckAt: sql.NewTimeOption(row.Host.LastAdvisoryCheckAt),
		StateUpdatedAt:      sql.NewTimeOption(row.StateUpdatedAt),
		PullLastRunAt:       sql.NewTimeOption(row.PullLastRunAt),
		PullLastRunStatus:   row.PullLastRunStatus.UnwrapOr(""),
		PullLastRunError:    row.PullLastRunError.UnwrapOr(""),
		Notes:               row.Host.Notes,
		Configuration:       configuration,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}
}

func mapHostWithStateByID(row sql.GetHostWithStateByIDRow) HostInfo {
	createdAt := row.Host.CreatedAt.Time.UTC()
	updatedAt := row.Host.UpdatedAt.Time.UTC()
	configuration := utils.None[SSHPullConfiguration]()
	if row.Host.OnboardingMode == "ssh" {
		frequency := int32(defaultSSHPullFrequency)
		if row.PullFrequencyMinutes != nil {
			frequency = *row.PullFrequencyMinutes
		}
		configuration = utils.Some(SSHPullConfiguration{
			Hostname:          row.PullHostname.UnwrapOr(""),
			SSHUser:           row.PullSshUser.UnwrapOr(""),
			FrequencyMinutes:  frequency,
			Onboarded:         row.Onboarded != nil && *row.Onboarded,
			UsesUniqueKeyPair: row.UsesUniqueKeyPair,
		})
	}
	return HostInfo{
		ID:                  row.Host.ID,
		OnboardingMode:      row.Host.OnboardingMode,
		ApprovalStatus:      row.Host.ApprovalStatus,
		DisplayName:         row.Host.DisplayName.UnwrapOr(""),
		Hostname:            row.Host.Hostname.UnwrapOr(""),
		IPAddress:           row.Host.IpAddress.UnwrapOr(""),
		OSFamily:            row.Host.OsFamily,
		OSName:              row.Host.OsName,
		OSMajor:             row.Host.OsMajor,
		OSVersion:           row.Host.OsVersion,
		Architecture:        row.Host.Architecture,
		Status:              row.Host.Status,
		OverallAction:       row.OverallAction,
		CriticalCount:       row.CriticalCount,
		ImportantCount:      row.ImportantCount,
		ModerateCount:       row.ModerateCount,
		ActionableCount:     row.ActionableCount,
		AvailableUpdates:    row.AvailableUpdates,
		NeedsReboot:         row.NeedsReboot,
		NeedsRestart:        row.NeedsRestart,
		NoFix:               row.NoFix,
		Unknown:             row.Unknown,
		LastSeenAt:          sql.NewTimeOption(row.Host.LastSeenAt),
		LastAdvisoryCheckAt: sql.NewTimeOption(row.Host.LastAdvisoryCheckAt),
		StateUpdatedAt:      sql.NewTimeOption(row.StateUpdatedAt),
		PullLastRunAt:       sql.NewTimeOption(row.PullLastRunAt),
		PullLastRunStatus:   row.PullLastRunStatus.UnwrapOr(""),
		PullLastRunError:    row.PullLastRunError.UnwrapOr(""),
		Notes:               row.Host.Notes,
		Configuration:       configuration,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}
}

func (s *hosts) RunSSHPull(ctx context.Context, actor ActorRef, hostID string) error {
	host, err := sql.Required(s.sql.GetHostByID(ctx, hostID))(apperr.ErrHostNotFound)
	if err != nil {
		return fmt.Errorf("get host: %w", err)
	}

	cfg, err := s.sql.GetSSHPullConfigByHostID(ctx, hostID)
	if err != nil {
		return fmt.Errorf("get ssh pull config: %w", err)
	}

	jobID := id.New("spjob")
	trigger := "scheduled"
	if actor.UserID != "system" {
		trigger = "manual"
	}
	auditStatus := "success"
	auditErrMsg := ""
	auditMetadata := map[string]any{
		"trigger": trigger,
		"job_id":  jobID,
	}
	defer func() {
		auditMetadata["status"] = auditStatus
		if auditErrMsg != "" {
			auditMetadata["error"] = auditErrMsg
		}
		s.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     auditLogActionSSHPull,
			TargetType: auditLogTargetTypeHost,
			TargetID:   hostID,
			Metadata:   auditMetadata,
			IPAddress:  actor.IP,
			UserAgent:  actor.UserAgent,
		})
	}()
	_, err = s.sql.InsertHostSSHPullJob(ctx, sql.InsertHostSSHPullJobParams{
		ID:        jobID,
		HostID:    hostID,
		Status:    "running",
		StartedAt: sql.TimestamptzFromTime(time.Now()),
	})
	if err != nil {
		return fmt.Errorf("insert host ssh pull job: %w", err)
	}

	updateJobResult := func(status string, errMsg string, res *SSHPullResult) error {
		tx, txErr := s.pool.BeginTx(ctx, pgx.TxOptions{})
		if txErr != nil {
			return fmt.Errorf("begin transaction: %w", txErr)
		}
		defer tx.Rollback(ctx) // nolint:errcheck

		queries := sql.New(tx)

		var snapshotID string
		var optErr utils.Option[string]
		if errMsg != "" {
			optErr = utils.Some(errMsg)
		}
		if _, updateJobErr := queries.UpdateHostSSHPullJob(ctx, sql.UpdateHostSSHPullJobParams{
			ID:          jobID,
			Status:      status,
			CompletedAt: sql.TimestamptzFromTime(time.Now()),
			Error:       optErr,
		}); updateJobErr != nil {
			return fmt.Errorf("update host ssh pull job: %w", updateJobErr)
		}

		statusOpt := utils.Some(status)
		errOpt := optErr

		if status == "success" && res != nil {
			snapshotRow, insertSnapshotErr := queries.InsertHostSnapshot(ctx, sql.InsertHostSnapshotParams{
				ID:                 id.New("snap"),
				HostID:             hostID,
				CollectedAt:        sql.TimestamptzFromTime(res.CollectedAt),
				Payload:            res.Payload,
				RunningKernelNevra: res.RunningKernel,
				BootTime:           res.BootTime.Map(sql.TimestamptzFromTime).UnwrapOrZero(),
				HasProcessData:     res.HasProcessData,
			})
			if insertSnapshotErr != nil {
				return fmt.Errorf("insert host snapshot: %w", insertSnapshotErr)
			}
			snapshotID = snapshotRow.ID

			if updateRunErr := queries.UpdateSSHPullRun(ctx, sql.UpdateSSHPullRunParams{
				ID:                hostID,
				PullLastRunAt:     sql.TimestamptzFromTime(res.CollectedAt),
				LastSnapshotID:    utils.Some(snapshotID),
				PullLastRunStatus: statusOpt,
				PullLastRunError:  errOpt,
				MachineID:         utils.NonZeroOption(res.MachineID),
				Hostname:          utils.NonZeroOption(res.Hostname),
				IpAddress:         utils.NonZeroOption(res.IPAddress),
				OsFamily:          res.OSFamily,
				OsName:            res.OSName,
				OsMajor:           res.OSMajor,
				OsVersion:         res.OSVersion,
				Architecture:      res.Architecture,
			}); updateRunErr != nil {
				return fmt.Errorf("update ssh pull run success: %w", updateRunErr)
			}

			if upsertStateErr := queries.UpsertHostCurrentState(ctx, sql.UpsertHostCurrentStateParams{
				HostID:           hostID,
				SnapshotID:       snapshotRow.ID,
				OverallAction:    res.OverallAction,
				CriticalCount:    res.CriticalCount,
				ImportantCount:   res.ImportantCount,
				ModerateCount:    res.ModerateCount,
				ActionableCount:  res.ActionableCount,
				AvailableUpdates: res.AvailableUpdates,
				NeedsReboot:      res.NeedsReboot,
				NeedsRestart:     res.NeedsRestart,
				NoFix:            res.NoFix,
				Unknown:          res.Unknown,
			}); upsertStateErr != nil {
				return fmt.Errorf("upsert host current state: %w", upsertStateErr)
			}
		} else {
			if updateRunErr := queries.UpdateSSHPullRun(ctx, sql.UpdateSSHPullRunParams{
				ID:                hostID,
				PullLastRunAt:     sql.TimestamptzFromTime(time.Now()),
				LastSnapshotID:    utils.None[string](),
				PullLastRunStatus: statusOpt,
				PullLastRunError:  errOpt,
				MachineID:         utils.None[string](),
				Hostname:          utils.None[string](),
				IpAddress:         utils.None[string](),
				OsFamily:          host.OsFamily,
				OsName:            host.OsName,
				OsMajor:           host.OsMajor,
				OsVersion:         host.OsVersion,
				Architecture:      host.Architecture,
			}); updateRunErr != nil {
				return fmt.Errorf("update ssh pull run failure: %w", updateRunErr)
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit job results update transaction: %w", err)
		}

		s.broker.Publish(events.NewHostSnapshotEvent(hostID))

		// Resolve and update advisory scope key (post-commit, outside the transaction using s.sql)
		if status == "success" && res != nil {
			scopeKey, err := s.advisoriesService.ResolveScopeKey(ctx, res.OSFamily, res.OSName, res.OSVersion, res.OSMajor, res.Architecture)
			if err == nil {
				var registerErr error
				if scopeKey != "" {
					registerErr = s.advisoriesService.RegisterScopeDemand(ctx, scopeKey)
					if registerErr != nil {
						utils.GetLogger(ctx).Warn("register scope demand failed", "error", registerErr)
					}
				}
				if registerErr == nil {
					err = s.sql.UpdateHostAdvisoryScopeKey(ctx, sql.UpdateHostAdvisoryScopeKeyParams{
						ID:               hostID,
						AdvisoryScopeKey: utils.NonZeroOption(scopeKey),
					})
					if err != nil {
						utils.GetLogger(ctx).Warn("update host advisory scope key failed", "error", err)
					}
				}
			} else {
				utils.GetLogger(ctx).Warn("resolve scope key failed", "error", err)
			}

			// Trigger MatchSnapshot
			if _, err := s.matcher.MatchSnapshot(ctx, hostID, snapshotID); err != nil {
				utils.GetLogger(ctx).Warn("matching snapshot failed", "host_id", hostID, "snapshot_id", snapshotID, "error", err)
			}
		}

		s.broker.Publish(events.NewHostsUpdatedEvent())

		return nil
	}

	var decryptedKey string
	if key, ok := cfg.PullPrivateKey.Get(); ok && key != "" {
		var err error
		decryptedKey, err = s.crypto.Decrypt(key)
		if err != nil {
			auditStatus = "failed"
			auditErrMsg = fmt.Sprintf("decrypt private key: %v", err)
			upErr := updateJobResult("failed", auditErrMsg, nil)
			if upErr != nil {
				return fmt.Errorf("decrypt private key failed (%v), and job update failed: %w", err, upErr)
			}
			return fmt.Errorf("decrypt private key: %w", err)
		}
	} else {
		globalKey, err := s.settingsService.GetGlobalSSHKeyPair(ctx)
		if err != nil {
			auditStatus = "failed"
			auditErrMsg = fmt.Sprintf("get global ssh key: %v", err)
			upErr := updateJobResult("failed", auditErrMsg, nil)
			if upErr != nil {
				return fmt.Errorf("get global ssh key failed (%v), and job update failed: %w", err, upErr)
			}
			return fmt.Errorf("get global ssh key: %w", err)
		}
		decryptedKey = globalKey.PrivateKey
	}

	sshHost := strings.TrimSpace(cfg.PullHostname)
	if sshHost == "" {
		auditStatus = "failed"
		auditErrMsg = "ssh pull hostname is empty"
		upErr := updateJobResult("failed", auditErrMsg, nil)
		if upErr != nil {
			return fmt.Errorf("empty ssh pull hostname, and job update failed: %w", upErr)
		}
		return fmt.Errorf("ssh pull hostname is empty")
	}

	address := sshHost
	if !strings.Contains(address, ":") {
		address = net.JoinHostPort(address, "22")
	}

	res, err := s.sshRunner.Collect(ctx, decryptedKey, cfg.PullSshUser.UnwrapOr(""), address)
	if err != nil {
		auditStatus = "failed"
		auditErrMsg = err.Error()
		upErr := updateJobResult("failed", auditErrMsg, nil)
		if upErr != nil {
			return fmt.Errorf("collect snapshot failed (%v), and job update failed: %w", err, upErr)
		}
		return fmt.Errorf("collect snapshot: %w", err)
	}

	if err := updateJobResult("success", "", &res); err != nil {
		auditStatus = "failed"
		auditErrMsg = err.Error()
		return fmt.Errorf("update job results for success: %w", err)
	}

	return nil
}

func (s *hosts) ListSSHPullJobs(ctx context.Context, hostID string) ([]HostSSHPullJobInfo, error) {
	rows, err := s.sql.ListHostSSHPullJobsByHostID(ctx, sql.ListHostSSHPullJobsByHostIDParams{
		HostID: hostID,
		Limit:  sshPullHistoryLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("list host ssh pull jobs by host id: %w", err)
	}

	jobs := make([]HostSSHPullJobInfo, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, HostSSHPullJobInfo{
			ID:          row.ID,
			HostID:      row.HostID,
			Status:      row.Status,
			StartedAt:   row.StartedAt.Time.UTC(),
			CompletedAt: sql.NewTimeOption(row.CompletedAt),
			Error:       row.Error,
		})
	}
	return jobs, nil
}

func (s *hosts) GetDashboardOverview(ctx context.Context) (DashboardOverview, error) {
	row, err := s.sql.GetDashboardOverview(ctx)
	if err != nil {
		return DashboardOverview{}, fmt.Errorf("get dashboard overview: %w", err)
	}
	recentRows, err := s.sql.ListRecentAdvisories(ctx)
	if err != nil {
		return DashboardOverview{}, fmt.Errorf("list recent advisories: %w", err)
	}

	recentAdvisories := make([]RecentAdvisory, len(recentRows))
	for i, r := range recentRows {
		recentAdvisories[i] = RecentAdvisory{
			ID:           r.ID,
			SourceSystem: r.SourceSystem,
			Vendor:       r.Vendor,
			AdvisoryType: r.AdvisoryType,
			Severity:     r.Severity.Ptr(),
			Summary:      r.Summary.Ptr(),
			PublishedAt:  r.PublishedAt.Ptr(),
		}
	}

	return DashboardOverview{
		TotalHosts:         row.TotalHosts,
		NeedAttention:      row.NeedAttention,
		RebootQueue:        row.RebootQueue,
		UnknownInvestigate: row.UnknownInvestigate,
		TotalAdvisories:    row.TotalAdvisories,
		TotalScopes:        row.TotalScopes,
		RecentAdvisories:   recentAdvisories,
	}, nil
}

func (s *hosts) GetCollectorScript(osFamily string) (string, error) {
	return collectorScriptForOSFamily(osFamily)
}

func (s *hosts) CreateManualHost(ctx context.Context, actor ActorRef, displayName string, hostname string) (HostInfo, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return HostInfo{}, fmt.Errorf("begin create manual host transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	queries := sql.New(tx)

	host, err := queries.InsertManualHost(ctx, sql.InsertManualHostParams{
		ID:          id.New("h"),
		DisplayName: utils.NonZeroOption(displayName),
		Hostname:    utils.NonZeroOption(hostname),
	})
	if err != nil {
		if sql.IsUniqueViolation(err, "hosts_display_name_unique_idx") {
			return HostInfo{}, apperr.ErrDuplicateHostDisplayName
		}
		return HostInfo{}, fmt.Errorf("insert manual host: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return HostInfo{}, fmt.Errorf("commit create manual host transaction: %w", err)
	}

	s.broker.Publish(events.NewHostsUpdatedEvent())
	s.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
		ActorID:    actor.UserID,
		ActorEmail: actor.Email,
		Action:     auditLogActionHostCreate,
		TargetType: auditLogTargetTypeHost,
		TargetID:   host.ID,
		Metadata: map[string]any{
			"onboarding_mode": "manual",
			"display_name":    displayName,
			"hostname":        hostname,
		},
		IPAddress: actor.IP,
		UserAgent: actor.UserAgent,
	})

	return mapHost(host, nil), nil
}

func (s *hosts) IngestManualReport(ctx context.Context, actor ActorRef, hostID string, reportContent []byte) error {
	host, err := sql.Required(s.sql.GetHostByID(ctx, hostID))(apperr.ErrHostNotFound)
	if err != nil {
		return fmt.Errorf("get host: %w", err)
	}

	if host.OnboardingMode != "manual" {
		return fmt.Errorf("host onboarding mode is not manual")
	}

	collectedAt := time.Now().UTC()
	res, err := ParseSSHPullReport(reportContent, collectedAt)
	if err != nil {
		return fmt.Errorf("parse manual report: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin manual report ingest transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	queries := sql.New(tx)

	bootTime := res.BootTime.Map(sql.TimestamptzFromTime).UnwrapOrZero()

	snapshotRow, err := queries.InsertHostSnapshot(ctx, sql.InsertHostSnapshotParams{
		ID:                 id.New("snap"),
		HostID:             hostID,
		CollectedAt:        sql.TimestamptzFromTime(collectedAt),
		Payload:            res.Payload,
		RunningKernelNevra: res.RunningKernel,
		BootTime:           bootTime,
		HasProcessData:     res.HasProcessData,
	})
	if err != nil {
		return fmt.Errorf("insert host snapshot: %w", err)
	}
	snapshotID := snapshotRow.ID

	_, err = queries.UpdateHostFromSnapshot(ctx, sql.UpdateHostFromSnapshotParams{
		ID:             hostID,
		MachineID:      utils.NonZeroOption(res.MachineID),
		Hostname:       utils.NonZeroOption(res.Hostname),
		IpAddress:      utils.NonZeroOption(res.IPAddress),
		OsFamily:       res.OSFamily,
		OsName:         res.OSName,
		OsMajor:        res.OSMajor,
		OsVersion:      res.OSVersion,
		Architecture:   res.Architecture,
		LastSeenAt:     sql.TimestamptzFromTime(collectedAt),
		LastSnapshotID: utils.NonZeroOption(snapshotID),
	})
	if err != nil {
		return fmt.Errorf("update host from snapshot: %w", err)
	}

	if err := queries.UpsertHostCurrentState(ctx, sql.UpsertHostCurrentStateParams{
		HostID:           hostID,
		SnapshotID:       snapshotID,
		OverallAction:    res.OverallAction,
		CriticalCount:    res.CriticalCount,
		ImportantCount:   res.ImportantCount,
		ModerateCount:    res.ModerateCount,
		ActionableCount:  res.ActionableCount,
		AvailableUpdates: res.AvailableUpdates,
		NeedsReboot:      res.NeedsReboot,
		NeedsRestart:     res.NeedsRestart,
		NoFix:            res.NoFix,
		Unknown:          res.Unknown,
	}); err != nil {
		return fmt.Errorf("upsert host current state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit manual report ingest transaction: %w", err)
	}

	// Post-commit work
	scopeKey, err := s.advisoriesService.ResolveScopeKey(ctx, res.OSFamily, res.OSName, res.OSVersion, res.OSMajor, res.Architecture)
	if err == nil {
		var registerErr error
		if scopeKey != "" {
			registerErr = s.advisoriesService.RegisterScopeDemand(ctx, scopeKey)
			if registerErr != nil {
				utils.GetLogger(ctx).Warn("register scope demand failed", "error", registerErr)
			}
		}
		if registerErr == nil {
			err = s.sql.UpdateHostAdvisoryScopeKey(ctx, sql.UpdateHostAdvisoryScopeKeyParams{
				ID:               hostID,
				AdvisoryScopeKey: utils.NonZeroOption(scopeKey),
			})
			if err != nil {
				utils.GetLogger(ctx).Warn("update host advisory scope key failed", "error", err)
			}
		}
	} else {
		utils.GetLogger(ctx).Warn("resolve scope key failed", "error", err)
	}

	// Trigger MatchSnapshot
	if _, err := s.matcher.MatchSnapshot(ctx, hostID, snapshotID); err != nil {
		utils.GetLogger(ctx).Warn("matching snapshot failed", "host_id", hostID, "snapshot_id", snapshotID, "error", err)
	}

	s.broker.Publish(events.NewHostsUpdatedEvent())
	s.broker.Publish(events.NewHostSnapshotEvent(hostID))
	s.audit.Record(ctx, AuditEvent{ // nolint: exhaustruct
		ActorID:    actor.UserID,
		ActorEmail: actor.Email,
		Action:     auditLogActionManualIngest,
		TargetType: auditLogTargetTypeHost,
		TargetID:   hostID,
		Metadata: map[string]any{
			"snapshot_id": snapshotID,
		},
		IPAddress: actor.IP,
		UserAgent: actor.UserAgent,
	})

	return nil
}
