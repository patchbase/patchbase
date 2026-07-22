// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/events"
	"go.patchbase.net/server/internal/sql"
	"go.patchbase.net/server/internal/sql/id"
	"go.patchbase.net/server/internal/utils"
)

const (
	auditLogActionLoginSuccess  = "auth.login.success"
	auditLogActionLoginFailure  = "auth.login.failure"
	auditLogActionProfileUpdate = "auth.profile.update"
	auditLogActionTokenCreate   = "host.registration_token.create" // nolint: gosec
	auditLogActionTokenRevoke   = "host.registration_token.revoke" // nolint: gosec
	auditLogActionHostCreate    = "host.create"
	auditLogActionHostUpdate    = "host.update"
	auditLogActionHostDelete    = "host.delete"
	auditLogActionHostApprove   = "host.approve"
	auditLogActionSSHPull       = "host.ssh.pull"
	auditLogActionSSHOnboard    = "host.ssh.onboard"
	auditLogActionManualIngest  = "host.manual.ingest"
	auditLogActionSettingUpdate = "settings.update"

	auditLogTargetTypeUser              = "user"
	auditLogTargetTypeRegistrationToken = "registration_token"
	auditLogTargetTypeHost              = "host"
	auditLogTargetTypeSetting           = "setting"

	defaultAuditLogListLimit = 50
	maxAuditLogListLimit     = 500
)

// AuditLogRecorder is the small surface other services consume to record
// audit events. Defined as an interface so tests can substitute a fake.
type AuditLogRecorder interface {
	Record(ctx context.Context, event AuditEvent)
}

type AuditLogService interface {
	AuditLogRecorder
	List(ctx context.Context, input ListAuditLogInput) (ListAuditLogResult, error)
}

type AuditEvent struct {
	ActorID    string
	ActorEmail string
	Action     string
	TargetType string
	TargetID   string
	Metadata   map[string]any
	IPAddress  string
	UserAgent  string
}

type ListAuditLogInput struct {
	Limit  utils.Option[int32]
	Offset utils.Option[int32]
	Action string
	Actor  string
	From   time.Time
	To     time.Time
}

// hasFilters reports whether any filter parameter was set. Used to decide
// whether the unfiltered or filtered sqlc query should run.
func (in ListAuditLogInput) hasFilters() bool {
	return in.Action != "" || in.Actor != "" || !in.From.IsZero() || !in.To.IsZero()
}

type ListAuditLogResult struct {
	Items []AuditLogInfo
	Total int64
}

type AuditLogInfo struct {
	ID         string               `json:"id"`
	ActorID    utils.Option[string] `json:"actor_id"`
	ActorEmail string               `json:"actor_email"`
	Action     string               `json:"action"`
	TargetType string               `json:"target_type"`
	TargetID   utils.Option[string] `json:"target_id"`
	Metadata   json.RawMessage      `json:"metadata"`
	IPAddress  utils.Option[string] `json:"ip_address"`
	UserAgent  utils.Option[string] `json:"user_agent"`
	CreatedAt  string               `json:"created_at"`
}

type auditLog struct {
	queries sql.Querier
	broker  events.Broker
}

func NewAuditLog(i do.Injector) (AuditLogService, error) {
	queries, err := do.Invoke[sql.Querier](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.Querier: %w", err)
	}
	broker, err := do.Invoke[events.Broker](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get events broker: %w", err)
	}
	return &auditLog{
		queries: queries,
		broker:  broker,
	}, nil
}

func (s *auditLog) Record(ctx context.Context, event AuditEvent) {
	actorID := utils.NonZeroOption(event.ActorID)
	targetID := utils.NonZeroOption(event.TargetID)

	var metadata []byte
	if len(event.Metadata) > 0 {
		var err error
		metadata, err = json.Marshal(event.Metadata)
		if err != nil {
			utils.GetLogger(ctx).
				ErrorContext(ctx, "failed to marshal audit log metadata", "action", event.Action, "error", err)
			return
		}
	}

	ipAddress := utils.NonZeroOption(event.IPAddress)
	userAgent := utils.NonZeroOption(event.UserAgent)

	if err := s.queries.InsertAuditLog(ctx, sql.InsertAuditLogParams{
		ID:         id.New("audit"),
		ActorID:    actorID,
		ActorEmail: event.ActorEmail,
		Action:     event.Action,
		TargetType: event.TargetType,
		TargetID:   targetID,
		Metadata:   metadata,
		IpAddress:  ipAddress,
		UserAgent:  userAgent,
	}); err != nil {
		utils.GetLogger(ctx).
			ErrorContext(ctx, "failed to insert audit log entry",
				"action", event.Action,
				"target_type", event.TargetType,
				"target_id", event.TargetID,
				slog.String("error", err.Error()))
		return
	}

	s.broker.Publish(events.NewAuditLogCreatedEvent())
}

func (s *auditLog) List(ctx context.Context, input ListAuditLogInput) (ListAuditLogResult, error) {
	limit := min(max(input.Limit.UnwrapOr(defaultAuditLogListLimit), 0), maxAuditLogListLimit)
	offset := max(input.Offset.UnwrapOr(0), 0)

	var (
		total int64
		rows  []sql.AuditLog
		err   error
	)

	if input.hasFilters() {
		actionOpt := utils.NonZeroOption(input.Action)
		actorOpt := utils.NonZeroOption(input.Actor)
		fromTS := sql.TimestamptzFromTime(input.From)
		toTS := sql.TimestamptzFromTime(input.To)

		total, err = s.queries.CountAuditLogsFiltered(ctx, sql.CountAuditLogsFilteredParams{
			Action:  actionOpt,
			ActorID: actorOpt,
			From:    fromTS,
			To:      toTS,
		})
		if err != nil {
			return ListAuditLogResult{}, fmt.Errorf("count audit logs: %w", err)
		}

		rows, err = s.queries.ListAuditLogsFiltered(ctx, sql.ListAuditLogsFilteredParams{
			Action:  actionOpt,
			ActorID: actorOpt,
			From:    fromTS,
			To:      toTS,
			Limit:   limit,
			Offset:  offset,
		})
		if err != nil {
			return ListAuditLogResult{}, fmt.Errorf("list audit logs: %w", err)
		}
	} else {
		total, err = s.queries.CountAuditLogs(ctx)
		if err != nil {
			return ListAuditLogResult{}, fmt.Errorf("count audit logs: %w", err)
		}

		rows, err = s.queries.ListAuditLogs(ctx, sql.ListAuditLogsParams{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return ListAuditLogResult{}, fmt.Errorf("list audit logs: %w", err)
		}
	}

	items := make([]AuditLogInfo, 0, len(rows))
	for _, row := range rows {
		var metadata json.RawMessage
		if len(row.Metadata) > 0 {
			metadata = json.RawMessage(row.Metadata)
		} else {
			metadata = json.RawMessage(`null`)
		}
		items = append(items, AuditLogInfo{
			ID:         row.ID,
			ActorID:    row.ActorID,
			ActorEmail: row.ActorEmail,
			Action:     row.Action,
			TargetType: row.TargetType,
			TargetID:   row.TargetID,
			Metadata:   metadata,
			IPAddress:  row.IpAddress,
			UserAgent:  row.UserAgent,
			CreatedAt:  row.CreatedAt.Time.UTC().Format("2006-01-02T15:04:05.000000Z"),
		})
	}

	return ListAuditLogResult{
		Items: items,
		Total: total,
	}, nil
}
