<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatsRow from '$lib/components/StatsRow.svelte';
	import { listHosts } from '$lib/api/hosts.js';
	import { relativeTime } from '$lib/format';
	import type { Host } from '$lib/types';

	let hosts = $state<Host[]>([]);

	$effect(() => {
		listHosts().then((data) => (hosts = data));
	});

	let rebootHosts = $derived(hosts.filter((h) => h.needs_reboot > 0));
	let rebootItems = $derived(
		hosts.reduce((sum, h) => sum + h.needs_reboot, 0),
	);

	let stats = $derived([
		{ label: 'Pending Reboots', value: rebootHosts.length, color: 'red' },
		{ label: 'Reboot Items', value: rebootItems, color: 'orange' },
	]);
</script>

<AppLayout page="reboots" title="Pending Reboots">
	<StatsRow {stats} />

	{#if rebootHosts.length === 0}
		<div class="empty-state">
			<p>No hosts require a reboot right now.</p>
		</div>
	{:else}
		<div class="host-grid">
			{#each rebootHosts as host (host.id)}
				<div class="host-card">
					<div class="host-card-header">
						<div class="host-card-name">
							<a href="/hosts/{host.id}">{host.display_name || host.hostname}</a>
						</div>
					</div>
					<div class="host-card-meta">
						{host.os_name} {host.os_version} &middot; {host.architecture}
					</div>
					<div class="host-card-signals">
						<span class="badge badge-red">
							<span class="badge-dot"></span>
							{host.needs_reboot} pending
						</span>
						{#if host.critical_count > 0}
							<span class="badge badge-red">{host.critical_count} critical</span>
						{/if}
					</div>
					<div class="host-card-footer">
						<span>{relativeTime(host.last_seen_at)}</span>
						<span>{relativeTime(host.last_advisory_check_at || null)}</span>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</AppLayout>

<style>
	.host-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
		gap: var(--space-md);
	}

	.host-card {
		display: flex;
		flex-direction: column;
		background: var(--bg-card);
		backdrop-filter: blur(12px);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 18px 20px;
		transition: all var(--transition-normal);
	}

	.host-card:hover {
		border-color: var(--border-light);
		box-shadow: var(--shadow-md);
		transform: translateY(-1px);
	}

	.host-card-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 8px;
	}

	.host-card-name {
		font-size: 15px;
		font-weight: 700;
		color: var(--text-primary);
	}

	.host-card-name a {
		color: var(--text-primary);
		text-decoration: none;
	}

	.host-card-name a:hover {
		color: var(--accent-light);
	}

	.host-card-meta {
		font-size: 12px;
		color: var(--text-dim);
		font-family: var(--font-mono);
		margin-bottom: 12px;
	}

	.host-card-signals {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		margin-bottom: 14px;
		flex: 1;
		align-content: flex-start;
	}

	.host-card-footer {
		display: flex;
		justify-content: space-between;
		font-size: 12px;
		color: var(--text-dim);
		padding-top: 10px;
		border-top: 1px solid var(--border);
	}
</style>