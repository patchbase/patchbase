import { authenticatedRequest } from "$lib/api/request.js";

export interface SettingsData {
  global_ssh_public_key: string;
}

export async function getSettings(): Promise<SettingsData> {
  return authenticatedRequest("/api/v1/settings");
}
