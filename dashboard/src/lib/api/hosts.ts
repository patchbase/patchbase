import { request } from "$lib/api/request.js";
import { getSession } from "$lib/auth/session.js";
import type { Host, HostSnapshot } from "$lib/types";

export interface RegistrationTokenInfo {
  id: string;
  name: string;
  created_by_user_id: string;
  created_at: string;
  revoked_at: string | null;
  last_used_at: string | null;
}

export interface CreatedRegistrationToken {
  id: string;
  name: string;
  token: string;
  created_at: string;
}

export interface CreateSSHHostPayload {
  display_name: string;
  hostname: string;
  ip_address?: string;
  ssh_user: string;
  frequency_minutes: number;
}

export interface CreatedSSHHost {
  host_id: string;
  public_key: string;
  approval_status: string;
  last_run_status: string;
  last_run_error: string;
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
      ...(init?.headers || {}),
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function listHosts(): Promise<Host[]> {
  return authenticatedRequest("/api/v1/hosts");
}

export async function getHost(id: string): Promise<Host> {
  return authenticatedRequest(`/api/v1/hosts/${id}`);
}

export async function getHostSnapshot(id: string): Promise<HostSnapshot> {
  return authenticatedRequest(`/api/v1/hosts/${id}/snapshot`);
}

export async function deleteHost(id: string): Promise<void> {
  await authenticatedRequest(`/api/v1/hosts/${id}`, {
    method: "DELETE",
  });
}

export async function listPendingHosts(): Promise<Host[]> {
  return authenticatedRequest("/api/v1/hosts/pending");
}

export async function approveHost(id: string): Promise<Host> {
  return authenticatedRequest(`/api/v1/hosts/${id}/approve`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: "{}",
  });
}

export async function listRegistrationTokens(): Promise<RegistrationTokenInfo[]> {
  return authenticatedRequest("/api/v1/hosts/tokens");
}

export async function createRegistrationToken(name: string): Promise<CreatedRegistrationToken> {
  return authenticatedRequest("/api/v1/hosts/tokens", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
}

export async function revokeRegistrationToken(id: string): Promise<void> {
  await authenticatedRequest(`/api/v1/hosts/tokens/${id}/revoke`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: "{}",
  });
}

export async function createSSHHost(payload: CreateSSHHostPayload): Promise<CreatedSSHHost> {
  return authenticatedRequest("/api/v1/hosts/ssh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}
