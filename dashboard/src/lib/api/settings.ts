import { authenticatedRequest } from "$lib/api/request.js";

export interface SettingsData {
  global_ssh_public_key: string;
  default_ssh_pull_user: string;
}

export async function getSettings(): Promise<SettingsData> {
  return authenticatedRequest("/api/v1/settings");
}

export interface UpdateSettingsRequest {
  default_ssh_pull_user?: string;
}

export async function updateSettings(req: UpdateSettingsRequest): Promise<void> {
  return authenticatedRequest("/api/v1/settings", {
    method: "PATCH",
    body: JSON.stringify(req),
  });
}
