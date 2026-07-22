// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package entities

import (
	"encoding/json"

	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/utils"
)

type AuditLogEntry struct {
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

type AuditLogPage struct {
	Items []AuditLogEntry `json:"items"`
	Total int64           `json:"total"`
}

func NewAuditLogEntry(value services.AuditLogInfo) AuditLogEntry {
	return AuditLogEntry{
		ID:         value.ID,
		ActorID:    value.ActorID,
		ActorEmail: value.ActorEmail,
		Action:     value.Action,
		TargetType: value.TargetType,
		TargetID:   value.TargetID,
		Metadata:   value.Metadata,
		IPAddress:  value.IPAddress,
		UserAgent:  value.UserAgent,
		CreatedAt:  value.CreatedAt,
	}
}

func NewAuditLogPage(value services.ListAuditLogResult) AuditLogPage {
	items := make([]AuditLogEntry, 0, len(value.Items))
	for _, item := range value.Items {
		items = append(items, NewAuditLogEntry(item))
	}
	return AuditLogPage{
		Items: items,
		Total: value.Total,
	}
}
