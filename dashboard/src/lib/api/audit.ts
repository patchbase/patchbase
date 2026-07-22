// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import { authenticatedRequest } from "$lib/api/request.js";

export interface AuditLogEntry {
  id: string;
  actor_id: string | null;
  actor_email: string;
  action: string;
  target_type: string;
  target_id: string | null;
  metadata: Record<string, unknown> | null;
  ip_address: string | null;
  user_agent: string | null;
  created_at: string;
}

export interface AuditLogPage {
  items: AuditLogEntry[];
  total: number;
}

export interface ListAuditLogParams {
  limit?: number;
  offset?: number;
  action?: string;
  actor?: string;
  from?: string;
  to?: string;
}

export async function listAuditEntries(params: ListAuditLogParams = {}): Promise<AuditLogPage> {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") {
      search.set(key, String(value));
    }
  }
  const query = search.toString();
  const path = query.length > 0 ? `/api/v1/audit-logs?${query}` : "/api/v1/audit-logs";
  return authenticatedRequest<AuditLogPage>(path);
}
