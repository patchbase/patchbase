// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import { request } from "$lib/api/request.js";

export interface SetupStatusResponse {
  completed: boolean;
}

export interface LoginResponse {
  access_token: string;
  setup_completed: boolean;
  password_reset_needed: boolean;
  user: {
    id: string;
    email: string;
    name: string;
    is_admin: boolean;
  };
}

const jsonHeaders = {
  "Content-Type": "application/json",
};

export async function getSetupStatus(): Promise<SetupStatusResponse> {
  return request("/api/v1/setup/status");
}

export async function login(email: string, password: string): Promise<LoginResponse> {
  return request("/api/v1/auth/login", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ email, password }),
  });
}

export async function completeSetup(
  accessToken: string,
  payload: { name: string; email: string; password: string },
): Promise<LoginResponse> {
  return request("/api/v1/setup/complete", {
    method: "POST",
    headers: {
      ...jsonHeaders,
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(payload),
  });
}
