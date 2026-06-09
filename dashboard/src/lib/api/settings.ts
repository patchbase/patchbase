import { authenticatedRequest } from "$lib/api/request.js";

export interface SMTPSettings {
  host: string;
  port: number;
  username: string;
  password?: string;
  from: string;
  report_hour: number;
}

export interface SettingsData {
  global_ssh_public_key: string;
  default_ssh_pull_user: string;
  ask_to_copy_public_key: boolean;
  smtp_settings: SMTPSettings;
  email_frequency: string;
}

export async function getSettings(): Promise<SettingsData> {
  return authenticatedRequest("/api/v1/settings");
}

export interface UpdateSettingsRequest {
  default_ssh_pull_user?: string;
  ask_to_copy_public_key?: boolean;
  smtp_settings?: SMTPSettings;
  email_frequency?: string;
}

export async function updateSettings(req: UpdateSettingsRequest): Promise<void> {
  return authenticatedRequest("/api/v1/settings", {
    method: "PATCH",
    body: JSON.stringify(req),
  });
}

export async function testEmail(to: string): Promise<void> {
  return authenticatedRequest("/api/v1/settings/test-email", {
    method: "POST",
    body: JSON.stringify({ to }),
  });
}

export async function sendReportNow(): Promise<void> {
  return authenticatedRequest("/api/v1/settings/send-report", {
    method: "POST",
  });
}
