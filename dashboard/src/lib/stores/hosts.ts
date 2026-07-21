// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import { writable } from "svelte/store";
import type { Host } from "$lib/types";

export const hosts = writable<Host[]>([]);
export const hostsConnected = writable<boolean>(false);
