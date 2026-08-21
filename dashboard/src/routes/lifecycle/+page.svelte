<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
	import { onMount } from 'svelte';
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatsRow from '$lib/components/StatsRow.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { listLifecycleHosts, getLifecycleCatalogSource } from '$lib/api/lifecycle.js';
	import { formatTime } from '$lib/format';
	import type { HostLifecycle, LifecycleCatalogSource } from '$lib/types';

	let loading = $state(true);
	let error = $state('');
	let hosts = $state<HostLifecycle[]>([]);
	let catalogSource = $state<LifecycleCatalogSource | null>(null);

	type StatusFilter = 'all' | 'standard-support-ended' | 'approaching-end-of-support' | 'supported' | 'extended-coverage-available' | 'unknown';
	let statusFilter = $state<StatusFilter>('all');

	const statusOrder: Record<string, number> = {
		'standard-support-ended': 0,
		'extended-coverage-available': 1,
		'approaching-end-of-support': 2,
		'unknown': 3,
		'supported': 4,
	};

	async function loadData(): Promise<void> {
		loading = true;
		error = '';
		try {
			const [hostData, catalogData] = await Promise.all([
				listLifecycleHosts(),
				getLifecycleCatalogSource().catch(() => null),
			]);
			hosts = hostData;
			catalogSource = catalogData;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load lifecycle data.';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void loadData();
	});

	let filteredHosts = $derived(
		statusFilter === 'all'
			? [...hosts].sort((a, b) => {
					const sa = statusOrder[a.lifecycle?.state ?? 'unknown'] ?? 5;
					const sb = statusOrder[b.lifecycle?.state ?? 'unknown'] ?? 5;
					if (sa !== sb) return sa - sb;
					return (a.display_name || a.hostname).localeCompare(b.display_name || b.hostname);
				})
			: hosts.filter((h) => (h.lifecycle?.state ?? 'unknown') === statusFilter),
	);

	let counts = $derived({
		supported: hosts.filter((h) => h.lifecycle?.state === 'supported').length,
		approaching: hosts.filter((h) => h.lifecycle?.state === 'approaching-end-of-support').length,
		ended: hosts.filter((h) => h.lifecycle?.state === 'standard-support-ended').length,
		extended: hosts.filter((h) => h.lifecycle?.state === 'extended-coverage-available').length,
		unknown: hosts.filter((h) => h.lifecycle?.state === 'unknown').length,
	});

	let stats = $derived([
		{ label: 'Supported', value: counts.supported, color: 'green' },
		{ label: 'Approaching EOL', value: counts.approaching, color: 'yellow' },
		{ label: 'Support Ended', value: counts.ended, color: 'red' },
		{ label: 'Extended Coverage', value: counts.extended, color: 'orange' },
	]);

	function hostLabel(host: HostLifecycle): string {
		return host.display_name || host.hostname || host.id;
	}

	function daysLabel(host: HostLifecycle): string {
		const days = host.lifecycle?.days_remaining;
		if (days === null || days === undefined) return '-';
		if (days === 0) return 'today';
		if (days > 0) return `${days} days remaining`;
		return `${Math.abs(days)} days ago`;
	}

	const filterOptions: { value: StatusFilter; label: string }[] = [
		{ value: 'all', label: `All (${hosts.length})` },
		{ value: 'standard-support-ended', label: `Support Ended (${counts.ended})` },
		{ value: 'extended-coverage-available', label: `Extended Coverage (${counts.extended})` },
		{ value: 'approaching-end-of-support', label: `Approaching EOL (${counts.approaching})` },
		{ value: 'supported', label: `Supported (${counts.supported})` },
		{ value: 'unknown', label: `Unknown (${counts.unknown})` },
	];
</script>

<AppLayout page="lifecycle" title="OS Lifecycle">
	<StatsRow {stats} />

	{#if catalogSource}
		<div class="catalog-source">
			<span class="catalog-label">Catalog generated:</span>
			<span class="catalog-value">{catalogSource.generated_at}</span>
			{#if catalogSource.source_urls.length > 0}
				<span class="catalog-label">Sources:</span>
				{#each catalogSource.source_urls as url, i}
					{#if i > 0}<span class="catalog-sep">,</span>{/if}
					<a href={url} target="_blank" rel="noopener" class="catalog-link">{url}</a>
				{/each}
			{/if}
		</div>
	{/if}

	{#if loading}
		<div class="empty-state"><p>Loading lifecycle data...</p></div>
	{:else if error}
		<div class="empty-state">
			<p>{error}</p>
			<button class="btn btn-secondary btn-sm" type="button" onclick={() => void loadData()}>Retry</button>
		</div>
	{:else if hosts.length === 0}
		<div class="empty-state"><p>No hosts registered.</p></div>
	{:else}
		<div class="filter-bar">
			{#each filterOptions as opt}
				<button
					type="button"
					class="filter-chip" class:active={statusFilter === opt.value}
					onclick={() => statusFilter = opt.value}
				>
					{opt.label}
				</button>
			{/each}
		</div>

		{#if filteredHosts.length === 0}
			<div class="empty-state"><p>No hosts match this filter.</p></div>
		{:else}
			<div class="table-container">
				<table>
					<thead>
						<tr>
							<th>Host</th>
							<th>Operating System</th>
							<th>Lifecycle</th>
							<th>Support End</th>
							<th>Days</th>
							<th>Extended Coverage</th>
							<th>Last Seen</th>
						</tr>
					</thead>
					<tbody>
						{#each filteredHosts as host (host.id)}
							<tr>
								<td><a href="/hosts/{host.id}">{hostLabel(host)}</a></td>
								<td class="mono">{host.os_name} {host.os_version}</td>
								<td><StatusBadge status={host.lifecycle?.state ?? 'unknown'} /></td>
								<td class="mono">{host.lifecycle?.standard_support_end || '-'}</td>
								<td>{daysLabel(host)}</td>
								<td>
									{#if host.lifecycle?.extended_coverage_note}
										<span class="ext-note" title={host.lifecycle.extended_coverage_note}>
											{host.lifecycle.extended_coverage_end || '-'}
										</span>
									{:else}
										-
									{/if}
								</td>
								<td>{formatTime(host.last_seen_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{/if}
</AppLayout>

<style>
	.catalog-source {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--text-dim);
		margin-bottom: var(--space-md);
		padding: 10px 16px;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
	}

	.catalog-label {
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		font-size: 11px;
	}

	.catalog-value {
		font-family: var(--font-mono);
	}

	.catalog-link {
		color: var(--accent-light);
		text-decoration: none;
		word-break: break-all;
	}

	.catalog-link:hover {
		text-decoration: underline;
	}

	.catalog-sep {
		color: var(--text-dim);
	}

	.filter-bar {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		margin-bottom: var(--space-md);
	}

	.filter-chip {
		padding: 6px 14px;
		font-size: 13px;
		font-weight: 500;
		border: 1px solid var(--border);
		background: var(--bg-primary);
		color: var(--text-secondary);
		border-radius: var(--radius-sm);
		cursor: pointer;
		transition: all var(--transition-fast);
	}

	.filter-chip:hover {
		border-color: var(--border-light);
		color: var(--text-primary);
	}

	.filter-chip.active {
		background: var(--accent-glow);
		border-color: var(--accent);
		color: var(--accent-light);
		font-weight: 600;
	}

	.ext-note {
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--orange);
		cursor: help;
	}
</style>