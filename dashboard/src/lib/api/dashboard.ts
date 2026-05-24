import { request } from "$lib/api/request.js";
import { getSession } from "$lib/auth/session.js";

export interface DashboardOverview {
  total_hosts: number;
  need_attention: number;
  reboot_queue: number;
  unknown_investigate: number;
  total_advisories: number;
  total_streams: number;
}

function requireAccessToken(): string {
  const session = getSession();
  if (!session?.accessToken) {
    throw new Error("Missing session. Please sign in again.");
  }
  return session.accessToken;
}

async function authenticatedRequest(path: string, init?: RequestInit): Promise<any> {
  const accessToken = requireAccessToken();
  return request(path, {
    ...init,
    headers: {
      ...init?.headers,
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function getDashboardOverview(): Promise<DashboardOverview> {
  return authenticatedRequest("/api/v1/dashboard/overview");
}
