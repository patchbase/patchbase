// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import { authenticatedRequest } from "$lib/api/request.js";

export interface ProfileResponse {
  access_token: string;
  user: {
    id: string;
    email: string;
    name: string;
  };
}

export interface UpdateProfileRequest {
  email?: string;
  current_password?: string;
  new_password?: string;
}

export async function getProfile(): Promise<ProfileResponse> {
  return authenticatedRequest("/api/v1/profile");
}

export async function updateProfile(req: UpdateProfileRequest): Promise<ProfileResponse> {
  return authenticatedRequest("/api/v1/profile", {
    method: "PATCH",
    body: JSON.stringify(req),
  });
}
