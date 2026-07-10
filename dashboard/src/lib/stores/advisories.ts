import { writable } from "svelte/store";
import type { AdvisoryScopeStatus, AdvisoryOverview } from "$lib/api/advisories";

export const advisoryScopes = writable<AdvisoryScopeStatus[]>([]);
export const advisoryOverview = writable<AdvisoryOverview | null>(null);
export const advisoriesConnected = writable<boolean>(false);
