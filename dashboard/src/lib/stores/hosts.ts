import { writable } from "svelte/store";
import type { Host } from "$lib/types";

export const hosts = writable<Host[]>([]);
export const hostsConnected = writable<boolean>(false);
