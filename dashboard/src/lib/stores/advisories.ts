// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import { writable } from "svelte/store";
import type { AdvisoryScopeStatus, AdvisoryOverview } from "$lib/api/advisories";

export const advisoryScopes = writable<AdvisoryScopeStatus[]>([]);
export const advisoryOverview = writable<AdvisoryOverview | null>(null);
export const advisoriesConnected = writable<boolean>(false);
