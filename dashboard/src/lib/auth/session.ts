// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import type { LoginResponse } from "$lib/api/auth.js";

export interface DashboardSession {
  accessToken: string;
  passwordResetNeeded: boolean;
  user: {
    id: string;
    email: string;
    name: string;
  };
}

const sessionStorageKey = "patchbase_dashboard_session";
const sessionChangedEvent = "patchbase:session-changed";

export function getSession(): DashboardSession | null {
  if (typeof window === "undefined") {
    return null;
  }

  const raw = window.localStorage.getItem(sessionStorageKey);
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as DashboardSession;
    if (!parsed?.accessToken || !parsed?.user?.id) {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

export function setSessionFromLogin(result: LoginResponse): DashboardSession {
  const session: DashboardSession = {
    accessToken: result.access_token,
    passwordResetNeeded: result.password_reset_needed,
    user: {
      id: result.user.id,
      email: result.user.email,
      name: result.user.name,
    },
  };

  setSession(session);
  return session;
}

export function setSession(session: DashboardSession): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(sessionStorageKey, JSON.stringify(session));
  window.dispatchEvent(new Event(sessionChangedEvent));
}

export function clearSession(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.removeItem(sessionStorageKey);
  window.dispatchEvent(new Event(sessionChangedEvent));
}
