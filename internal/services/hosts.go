package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do/v2"
	agentpb "go.patchbase.net/proto/agent"
	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	"go.patchbase.net/server/internal/utils"
	"google.golang.org/protobuf/proto"
)

const (
	defaultNextCheckInSeconds = int32(21600)
	defaultSSHPullFrequency   = 360
)

var (
	ErrInvalidRegistrationToken = errors.New("invalid registration token")
	ErrInvalidHostAccessToken   = errors.New("invalid host access token")
	ErrHostNotApproved          = errors.New("host is not approved")
	ErrHostNotFound             = errors.New("host not found")
	ErrTokenAlreadyRevoked      = errors.New("registration token already revoked")
	ErrInvalidSnapshotPayload   = errors.New("invalid snapshot payload")
)

type Hosts interface {
	CreateRegistrationToken(ctx context.Context, userID string, name string) (CreatedRegistrationToken, error)
	ListRegistrationTokens(ctx context.Context) ([]RegistrationTokenInfo, error)
	RevokeRegistrationToken(ctx context.Context, tokenID string) error
	RegisterAgentHost(ctx context.Context, input *agentpb.RegisterHostRequest) (*agentpb.RegisterHostResponse, error)
	IngestAgentSnapshot(ctx context.Context, hostAccessToken string, payload *agentpb.AgentSnapshot) (*agentpb.SyncResponse, error)
	ListPendingHosts(ctx context.Context) ([]HostInfo, error)
	ApproveHost(ctx context.Context, hostID string) (HostInfo, error)
	DeleteHost(ctx context.Context, hostID string) error
	CreateSSHHost(ctx context.Context, input CreateSSHHostInput) (CreateSSHHostResult, error)
	ListHosts(ctx context.Context) ([]HostInfo, error)
	GetHost(ctx context.Context, hostID string) (HostInfo, error)
	GetLatestSnapshot(ctx context.Context, hostID string) (HostSnapshotInfo, error)
}

type hosts struct {
	pool      *pgxpool.Pool
	sql       sql.Querier
	random    utils.RandomStringGenerator
	sshRunner SSHPullRunner
	crypto    utils.Crypto
}

type CreatedRegistrationToken struct {
	ID        string
	Name      string
	Token     string
	CreatedAt time.Time
}

type RegistrationTokenInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedBy  string     `json:"created_by_user_id"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

type HostInfo struct {
	ID                  string     `json:"id"`
	OnboardingMode      string     `json:"onboarding_mode"`
	ApprovalStatus      string     `json:"approval_status"`
	DisplayName         string     `json:"display_name"`
	Hostname            string     `json:"hostname"`
	IPAddress           string     `json:"ip_address"`
	OSFamily            string     `json:"os_family"`
	OSName              string     `json:"os_name"`
	OSMajor             int32      `json:"os_major"`
	OSVersion           string     `json:"os_version"`
	Architecture        string     `json:"architecture"`
	Status              string     `json:"status"`
	OverallAction       string     `json:"overall_action"`
	CriticalCount       int32      `json:"critical_count"`
	ImportantCount      int32      `json:"important_count"`
	ModerateCount       int32      `json:"moderate_count"`
	ActionableCount     int32      `json:"actionable_count"`
	AvailableUpdates    int32      `json:"available_updates"`
	NeedsReboot         int32      `json:"needs_reboot"`
	NeedsRestart        int32      `json:"needs_restart"`
	NoFix               int32      `json:"no_fix"`
	Unknown             int32      `json:"unknown"`
	LastSeenAt          *time.Time `json:"last_seen_at"`
	LastAdvisoryCheckAt *time.Time `json:"last_advisory_check_at"`
	StateUpdatedAt      *time.Time `json:"state_updated_at"`
	PullLastRunAt       *time.Time `json:"pull_last_run_at"`
	PullLastRunStatus   string     `json:"pull_last_run_status"`
	PullLastRunError    string     `json:"pull_last_run_error"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type HostSnapshotInfo struct {
	ID                 string     `json:"id"`
	HostID             string     `json:"host_id"`
	CollectedAt        time.Time  `json:"collected_at"`
	ReceivedAt         time.Time  `json:"received_at"`
	RunningKernelNevra string     `json:"running_kernel_nevra"`
	BootTime           *time.Time `json:"boot_time"`
	HasProcessData     bool       `json:"has_process_data"`
}

type CreateSSHHostInput struct {
	DisplayName      string
	Hostname         string
	IPAddress        string
	SSHUser          string
	FrequencyMinutes int32
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

	return &hosts{
		pool:      pool,
		sql:       queries,
		random:    random,
		sshRunner: defaultSSHPullRunner{},
		crypto:    crypto,
	}, nil
}

func (s *hosts) CreateRegistrationToken(ctx context.Context, userID string, name string) (CreatedRegistrationToken, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		trimmed = "Registration token"
	}

	plain := "pb_reg_" + s.random.Hex(24)
	created, err := s.sql.InsertRegistrationToken(ctx, sql.InsertRegistrationTokenParams{
		ID:              id.New("rtok"),
		Name:            trimmed,
		TokenHash:       hashToken(plain),
		CreatedByUserID: userID,
	})
	if err != nil {
		return CreatedRegistrationToken{}, fmt.Errorf("insert registration token: %w", err)
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
			ID:         row.ID,
			Name:       row.Name,
			CreatedBy:  row.CreatedByUserID,
			CreatedAt:  row.CreatedAt.Time.UTC(),
			RevokedAt:  toTimePtr(row.RevokedAt),
			LastUsedAt: toTimePtr(row.LastUsedAt),
		})
	}
	return items, nil
}

func (s *hosts) RevokeRegistrationToken(ctx context.Context, tokenID string) error {
	if _, err := s.sql.RevokeRegistrationToken(ctx, tokenID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTokenAlreadyRevoked
		}
		return fmt.Errorf("revoke registration token: %w", err)
	}
	return nil
}

func (s *hosts) RegisterAgentHost(ctx context.Context, input *agentpb.RegisterHostRequest) (*agentpb.RegisterHostResponse, error) {
	registrationToken := strings.TrimSpace(input.RegistrationToken)
	if registrationToken == "" {
		return nil, ErrInvalidRegistrationToken
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin register agent host transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	queries := sql.New(tx)
	regToken, err := queries.GetActiveRegistrationTokenByHash(ctx, hashToken(registrationToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidRegistrationToken
		}
		return nil, fmt.Errorf("get active registration token: %w", err)
	}

	hostname := strings.TrimSpace(input.Hostname)
	machineID := strings.TrimSpace(input.MachineId)
	ipAddress := strings.TrimSpace(input.Metadata.IpAddress)
	osName := strings.TrimSpace(input.Metadata.OsName)
	if osName == "" {
		osName = "Unknown"
	}
	osVersion := strings.TrimSpace(input.Metadata.OsVersion)
	if osVersion == "" {
		osVersion = "unknown"
	}
	architecture := normalizeRegistrationArchitecture(strings.TrimSpace(input.Metadata.Architecture))
	if architecture == "" {
		architecture = "unknown"
	}
	displayName := utils.None[string]()
	if hostname != "" {
		displayName = utils.Some(hostname)
	}

	host, err := queries.InsertAgentHost(ctx, sql.InsertAgentHostParams{
		ID:           id.New("h"),
		DisplayName:  displayName,
		MachineID:    optionString(machineID),
		Hostname:     optionString(hostname),
		IpAddress:    optionString(ipAddress),
		OsName:       osName,
		OsVersion:    osVersion,
		Architecture: architecture,
	})
	if err != nil {
		return nil, fmt.Errorf("insert agent host: %w", err)
	}

	hostAccessToken := "pb_host_" + s.random.Hex(24)
	_, err = queries.InsertHostAccessToken(ctx, sql.InsertHostAccessTokenParams{
		ID:        id.New("htok"),
		HostID:    host.ID,
		TokenHash: hashToken(hostAccessToken),
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

	return &agentpb.RegisterHostResponse{
		HostId:          host.ID,
		HostAccessToken: hostAccessToken,
		ApprovalStatus:  host.ApprovalStatus,
	}, nil
}

func (s *hosts) IngestAgentSnapshot(ctx context.Context, hostAccessToken string, snapshot *agentpb.AgentSnapshot) (*agentpb.SyncResponse, error) {
	trimmedToken := strings.TrimSpace(hostAccessToken)
	if trimmedToken == "" {
		return nil, ErrInvalidHostAccessToken
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin snapshot transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	queries := sql.New(tx)
	tokenRow, err := queries.GetActiveHostAccessTokenByHash(ctx, hashToken(trimmedToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidHostAccessToken
		}
		return nil, fmt.Errorf("get active host access token: %w", err)
	}

	host, err := queries.GetHostByID(ctx, tokenRow.HostID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrHostNotFound
		}
		return nil, fmt.Errorf("get host by id: %w", err)
	}
	if host.ApprovalStatus != "approved" {
		return nil, ErrHostNotApproved
	}

	collectedAt := time.Now().UTC()
	if snapshot.GetSentAt() != nil {
		collectedAt = snapshot.GetSentAt().AsTime().UTC()
	}

	bootTime := pgtype.Timestamptz{}
	if snapshot.GetHost() != nil && snapshot.GetHost().GetBootTime() != nil {
		bootTime = pgTime(snapshot.GetHost().GetBootTime().AsTime().UTC())
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
		CollectedAt:        pgTime(collectedAt),
		Payload:            payload,
		RunningKernelNevra: runningKernel,
		BootTime:           bootTime,
		HasProcessData:     hasProcessData,
	})
	if err != nil {
		return nil, fmt.Errorf("insert host snapshot: %w", err)
	}

	hostPayload := snapshot.GetHost()
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
		MachineID:      optionString(machineID),
		Hostname:       optionString(hostname),
		IpAddress:      optionString(ipAddress),
		OsFamily:       osFamily,
		OsName:         osName,
		OsMajor:        osMajor,
		OsVersion:      osVersion,
		Architecture:   architecture,
		LastSeenAt:     pgTime(collectedAt),
		LastSnapshotID: optionString(snapshotRow.ID),
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

func (s *hosts) ApproveHost(ctx context.Context, hostID string) (HostInfo, error) {
	host, err := s.sql.ApproveHostByID(ctx, hostID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return HostInfo{}, ErrHostNotFound
		}
		return HostInfo{}, fmt.Errorf("approve host: %w", err)
	}
	return mapHost(host, nil), nil
}

func (s *hosts) CreateSSHHost(ctx context.Context, input CreateSSHHostInput) (CreateSSHHostResult, error) {
	hostname := strings.TrimSpace(input.Hostname)
	sshUser := strings.TrimSpace(input.SSHUser)
	if hostname == "" {
		return CreateSSHHostResult{}, fmt.Errorf("hostname is required")
	}
	if sshUser == "" {
		return CreateSSHHostResult{}, fmt.Errorf("ssh user is required")
	}
	frequency := input.FrequencyMinutes
	if frequency <= 0 {
		frequency = defaultSSHPullFrequency
	}

	publicKey, privateKey, err := utils.GenerateSSHKeyPair()
	if err != nil {
		return CreateSSHHostResult{}, fmt.Errorf("generate ssh key pair: %w", err)
	}

	encryptedPrivateKey, err := s.crypto.Encrypt(privateKey)
	if err != nil {
		return CreateSSHHostResult{}, fmt.Errorf("encrypt private key: %w", err)
	}

	host, err := s.sql.InsertSSHHost(ctx, sql.InsertSSHHostParams{
		ID:                   id.New("h"),
		DisplayName:          optionString(strings.TrimSpace(input.DisplayName)),
		Hostname:             optionString(hostname),
		IpAddress:            optionString(strings.TrimSpace(input.IPAddress)),
		PullSshUser:          optionString(sshUser),
		PullFrequencyMinutes: &frequency,
		PullPublicKey:        optionString(publicKey),
		PullPrivateKey:       optionString(encryptedPrivateKey),
	})
	if err != nil {
		return CreateSSHHostResult{}, fmt.Errorf("insert ssh host: %w", err)
	}

	address := hostname
	if !strings.Contains(address, ":") {
		address = net.JoinHostPort(hostname, "22")
	}
	runStatus, runError := s.sshRunner.TryConnect(ctx, address)

	return CreateSSHHostResult{
		HostID:         host.Host.ID,
		PublicKey:      publicKey,
		ApprovalStatus: host.Host.ApprovalStatus,
		LastRunStatus:  runStatus,
		LastRunError:   runError,
	}, nil
}

func (s *hosts) ListHosts(ctx context.Context) ([]HostInfo, error) {
	rows, err := s.sql.ListHostsWithState(ctx)
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}
	items := make([]HostInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapHostWithState(row))
	}
	return items, nil
}

func (s *hosts) GetHost(ctx context.Context, hostID string) (HostInfo, error) {
	row, err := s.sql.GetHostWithStateByID(ctx, hostID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return HostInfo{}, ErrHostNotFound
		}
		return HostInfo{}, fmt.Errorf("get host: %w", err)
	}
	return mapHostWithStateByID(row), nil
}

func (s *hosts) GetLatestSnapshot(ctx context.Context, hostID string) (HostSnapshotInfo, error) {
	row, err := s.sql.GetLatestHostSnapshotByHostID(ctx, hostID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return HostSnapshotInfo{}, ErrHostNotFound
		}
		return HostSnapshotInfo{}, fmt.Errorf("get latest snapshot: %w", err)
	}
	return HostSnapshotInfo{
		ID:                 row.ID,
		HostID:             row.HostID,
		CollectedAt:        row.CollectedAt.Time.UTC(),
		ReceivedAt:         row.ReceivedAt.Time.UTC(),
		RunningKernelNevra: row.RunningKernelNevra,
		BootTime:           toTimePtr(row.BootTime),
		HasProcessData:     row.HasProcessData,
	}, nil
}

func (s *hosts) DeleteHost(ctx context.Context, hostID string) error {
	trimmedHostID := strings.TrimSpace(hostID)
	if trimmedHostID == "" {
		return ErrHostNotFound
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin delete host transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	queries := sql.New(tx)
	_, err = queries.GetHostByID(ctx, trimmedHostID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrHostNotFound
		}
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
	if _, err := queries.DeleteHostByID(ctx, trimmedHostID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrHostNotFound
		}
		return fmt.Errorf("delete host: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete host transaction: %w", err)
	}

	return nil
}

func normalizeOSFamily(value agentpb.OsFamily) string {
	switch value {
	case agentpb.OsFamily_OS_FAMILY_RPM:
		return "rpm"
	case agentpb.OsFamily_OS_FAMILY_APT:
		return "apt"
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
	default:
		return "unknown"
	}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func optionString(value string) utils.Option[string] {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return utils.None[string]()
	}
	return utils.Some(trimmed)
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

func pgTime(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func toTimePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	value := ts.Time.UTC()
	return &value
}

func mapHost(host sql.Host, state *sql.HostCurrentState) HostInfo {
	overallAction := "none"
	criticalCount := int32(0)
	importantCount := int32(0)
	moderateCount := int32(0)
	actionableCount := int32(0)
	availableUpdates := int32(0)
	needsReboot := int32(0)
	needsRestart := int32(0)
	noFix := int32(0)
	unknown := int32(0)
	var stateUpdatedAt *time.Time
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
		stateUpdatedAt = toTimePtr(state.UpdatedAt)
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
		LastSeenAt:          toTimePtr(host.LastSeenAt),
		LastAdvisoryCheckAt: toTimePtr(host.LastAdvisoryCheckAt),
		StateUpdatedAt:      stateUpdatedAt,
		PullLastRunAt:       nil,
		PullLastRunStatus:   "",
		PullLastRunError:    "",
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}
}

func mapHostWithState(row sql.ListHostsWithStateRow) HostInfo {
	createdAt := row.Host.CreatedAt.Time.UTC()
	updatedAt := row.Host.UpdatedAt.Time.UTC()
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
		LastSeenAt:          toTimePtr(row.Host.LastSeenAt),
		LastAdvisoryCheckAt: toTimePtr(row.Host.LastAdvisoryCheckAt),
		StateUpdatedAt:      toTimePtr(row.StateUpdatedAt),
		PullLastRunAt:       toTimePtr(row.PullLastRunAt),
		PullLastRunStatus:   row.PullLastRunStatus.UnwrapOr(""),
		PullLastRunError:    row.PullLastRunError.UnwrapOr(""),
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}
}

func mapHostWithStateByID(row sql.GetHostWithStateByIDRow) HostInfo {
	createdAt := row.Host.CreatedAt.Time.UTC()
	updatedAt := row.Host.UpdatedAt.Time.UTC()
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
		LastSeenAt:          toTimePtr(row.Host.LastSeenAt),
		LastAdvisoryCheckAt: toTimePtr(row.Host.LastAdvisoryCheckAt),
		StateUpdatedAt:      toTimePtr(row.StateUpdatedAt),
		PullLastRunAt:       toTimePtr(row.PullLastRunAt),
		PullLastRunStatus:   row.PullLastRunStatus.UnwrapOr(""),
		PullLastRunError:    row.PullLastRunError.UnwrapOr(""),
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}
}
