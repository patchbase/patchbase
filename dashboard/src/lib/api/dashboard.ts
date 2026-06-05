import { authenticatedRequest } from "$lib/api/request.js";

export interface DashboardOverview {
  total_hosts: number;
  need_attention: number;
  reboot_queue: number;
  unknown_investigate: number;
  total_advisories: number;
  total_streams: number;
}

export async function getDashboardOverview(): Promise<DashboardOverview> {
  return authenticatedRequest("/api/v1/dashboard/overview");
}
