import { clearSession } from "$lib/auth/session.js";

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
    "error" in data &&
    typeof data.error === "string"
  ) {
    return data.error;
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
