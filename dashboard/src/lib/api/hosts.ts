import { authenticatedRequest } from "$lib/api/request.js";
import type {
  Host,
  HostSnapshot,
  HostPullJob,
  HostKernelPosture,
  MatcherDecisionGroup,
} from "$lib/types";

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
  unique_key_pair?: boolean;
}

export interface CreatedSSHHost {
  host_id: string;
  public_key: string;
  approval_status: string;
  last_run_status: string;
  last_run_error: string;
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

export async function listPullJobs(id: string): Promise<HostPullJob[]> {
  return authenticatedRequest(`/api/v1/hosts/${id}/pull-jobs`);
}

export async function runPullNow(id: string): Promise<void> {
  await authenticatedRequest(`/api/v1/hosts/${id}/pull-now`, {
    method: "POST",
  });
}

export async function onboardSSHHost(id: string): Promise<void> {
  await authenticatedRequest(`/api/v1/hosts/${id}/onboard-ssh`, {
    method: "POST",
  });
}

export async function getHostVulnerablePackages(id: string): Promise<MatcherDecisionGroup[]> {
  return authenticatedRequest(`/api/v1/hosts/${id}/packages/vulnerable`);
}

export async function getHostUpgradablePackages(id: string): Promise<MatcherDecisionGroup[]> {
  return authenticatedRequest(`/api/v1/hosts/${id}/packages/upgradable`);
}

export async function getHostKernelPosture(id: string): Promise<HostKernelPosture> {
  return authenticatedRequest(`/api/v1/hosts/${id}/kernel-posture`);
}

export async function createManualHost(
  displayName: string,
  hostname: string,
): Promise<{ host_id: string; approval_status: string }> {
  return authenticatedRequest("/api/v1/hosts/manual", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ display_name: displayName, hostname }),
  });
}

export async function ingestManualReport(hostId: string, reportContent: string): Promise<void> {
  await authenticatedRequest(`/api/v1/hosts/${hostId}/report`, {
    method: "POST",
    body: reportContent,
  });
}
