// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import { getSession, clearSession } from "$lib/auth/session.js";

export function requireAccessToken(): string {
  const session = getSession();
  if (!session?.accessToken) {
    throw new Error("Missing session. Please sign in again.");
  }
  return session.accessToken;
}

export async function authenticatedRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const accessToken = requireAccessToken();
  return request(path, {
    ...init,
    headers: {
      ...init?.headers,
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init);
  const data = await response.json().catch(() => null);

  if (!response.ok) {
    const errorMessage = getErrorMessage(data);
    handleInvalidAccessToken(response.status, errorMessage);
    throw new Error(errorMessage);
  }

  return data as T;
}

function getErrorMessage(data: unknown): string {
  if (
    typeof data === "object" &&
    data !== null &&
    "message" in data &&
    typeof data.message === "string"
  ) {
    return data.message;
  }
  return "Request failed";
}

function handleInvalidAccessToken(status: number, errorMessage: string): void {
  if (status !== 401) {
    return;
  }

  if (errorMessage.trim().toLowerCase() !== "invalid access token") {
    return;
  }

  clearSession();
}
