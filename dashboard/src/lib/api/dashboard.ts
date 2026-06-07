import { authenticatedRequest } from "$lib/api/request.js";

export interface DashboardOverview {
  total_hosts: number;
  need_attention: number;
  reboot_queue: number;
  unknown_investigate: number;
  total_advisories: number;
  total_scopes: number;
  recent_advisories: RecentAdvisory[];
}

export interface RecentAdvisory {
  id: string;
  source_system: string;
  vendor: string;
  advisory_type: string;
  severity: string | null;
  summary: string | null;
  published_at: string | null;
}

export async function getDashboardOverview(): Promise<DashboardOverview> {
  return authenticatedRequest("/api/v1/dashboard/overview");
}
