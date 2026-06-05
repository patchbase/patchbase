import { authenticatedRequest } from "$lib/api/request.js";

export interface AdvisoryScopeStatus {
  scope_key: string;
  status: string;
  last_sync_at: string | null;
  last_success_at: string | null;
  last_error: string | null;
  advisory_count: number;
  sha256: string | null;
  size_bytes: number;
  local_path: string | null;
  next_refresh_at: string | null;
  host_usage_count: number;
}

export interface AdvisoryOverview {
  total_advisories: number;
  total_scopes: number;
  synced_scopes: number;
}

export async function listAdvisoryScopes(): Promise<AdvisoryScopeStatus[]> {
  return authenticatedRequest("/api/v1/advisories/scopes");
}

export async function triggerAdvisorySync(scopeKey: string): Promise<{ status: string }> {
  return authenticatedRequest(`/api/v1/advisories/scopes/${encodeURIComponent(scopeKey)}/sync`, {
    method: "POST",
  });
}

export async function getAdvisoryOverview(): Promise<AdvisoryOverview> {
  return authenticatedRequest("/api/v1/advisories/overview");
}
