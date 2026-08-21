// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import { authenticatedRequest } from "$lib/api/request.js";
import type { HostLifecycle, LifecycleCatalogSource } from "$lib/types";

export async function listLifecycleHosts(): Promise<HostLifecycle[]> {
  return authenticatedRequest("/api/v1/lifecycle/hosts");
}

export async function getLifecycleCatalogSource(): Promise<LifecycleCatalogSource> {
  return authenticatedRequest("/api/v1/lifecycle/catalog");
}
