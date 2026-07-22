<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
	import { onMount } from 'svelte';
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatsRow from '$lib/components/StatsRow.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { listAdvisoryScopes, triggerAdvisorySync, getAdvisoryOverview } from '$lib/api/advisories.js';
	import { relativeTime, formatBytes } from '$lib/format';
	import { advisoryScopes, advisoryOverview, advisoriesConnected } from '$lib/stores/advisories';
	import { createPollingFallback } from '$lib/ws/fallback';

	let loading = $state(true);
	let error = $state('');
	let syncingScopes = $state<Record<string, boolean>>({});

	async function loadAdvisories(silent = false): Promise<void> {
		if (!silent) {
			loading = true;
			error = '';
		}
		try {
			const [newScopes, newOverview] = await Promise.all([
				listAdvisoryScopes(),
				getAdvisoryOverview()
			]);
			advisoryScopes.set(newScopes);
			advisoryOverview.set(newOverview);
		} catch (err) {
			if (!silent) {
				error = err instanceof Error ? err.message : 'Failed to load advisories.';
			} else {
				console.error('Failed to poll advisories:', err);
			}
		} finally {
			if (!silent) {
				loading = false;
			}
		}
	}

	onMount(() => {
		if ($advisoryScopes.length === 0 && $advisoryOverview === null) {
			void loadAdvisories();
		} else {
			loading = false;
		}

		let fallbackTimer: ReturnType<typeof setTimeout> | undefined;
		const fallback = createPollingFallback(() => loadAdvisories(true), 5000);
		const unsubConnected = advisoriesConnected.subscribe((connected) => {
			if (!connected) {
				fallbackTimer = setTimeout(() => { fallback.start(); }, 10000);
			} else {
				clearTimeout(fallbackTimer);
				fallback.stop();
			}
		});

		return () => {
			unsubConnected();
			clearTimeout(fallbackTimer);
			fallback.stop();
		};
	});

	async function handleSync(scopeKey: string): Promise<void> {
		syncingScopes = { ...syncingScopes, [scopeKey]: true };
		try {
			await triggerAdvisorySync(scopeKey);
			advisoryScopes.update(scopes => {
				const idx = scopes.findIndex((s) => s.scope_key === scopeKey);
				if (idx !== -1) {
					scopes[idx] = { ...scopes[idx], status: 'running' };
				}
				return scopes;
			});
			await loadAdvisories(true);
		} catch (err) {
			alert(err instanceof Error ? err.message : 'Failed to trigger sync.');
		} finally {
			syncingScopes = { ...syncingScopes, [scopeKey]: false };
		}
	}

	let activeHosts = $derived($advisoryScopes.reduce((sum, s) => sum + s.host_usage_count, 0));

	let stats = $derived([
		{ label: 'Total Advisories', value: $advisoryOverview?.total_advisories ?? 0, color: 'accent' },
		{ label: 'Demanded Scopes', value: $advisoryOverview?.total_scopes ?? 0, color: 'blue' },
		{ label: 'Synced Scopes', value: $advisoryOverview?.synced_scopes ?? 0, color: 'green' },
		{ label: 'Active Hosts', value: activeHosts, color: 'purple' }
	]);
</script>

<AppLayout page="advisories" title="Advisories">
	{#if error}
		<div class="alert-error">
			{error}
		</div>
	{/if}

	<StatsRow {stats} />

	<div class="table-container">
		<div class="table-header">
			<h2>Scope Registry Status</h2>
		</div>

		{#if loading}
			<div class="empty-state">
				<p>Loading advisories...</p>
			</div>
		{:else}
			{#if $advisoryScopes.length === 0}
				<div class="empty-state">
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<ellipse cx="12" cy="5" rx="9" ry="3"></ellipse>
						<path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"></path>
						<path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3"></path>
					</svg>
					<p class="empty-title">No active advisory scopes</p>
					<p class="empty-desc">Scopes are automatically added and synced when your hosts check in.</p>
				</div>
			{:else}
				<table>
					<thead>
						<tr>
							<th>Scope</th>
							<th>Status</th>
							<th>Advisories</th>
							<th>Size</th>
							<th>SHA256</th>
							<th>Active Hosts</th>
							<th>Last Sync Success</th>
							<th>Next Refresh</th>
							<th class="action-cell">Action</th>
						</tr>
					</thead>
					<tbody>
						{#each $advisoryScopes as scope (scope.scope_key)}
							<tr>
								<td class="mono scope-key">{scope.scope_key}</td>
								<td>
									<StatusBadge status={scope.status} />
								</td>
								<td class="mono">{scope.advisory_count}</td>
								<td class="mono">{formatBytes(scope.size_bytes)}</td>
								<td class="mono sha-text" title={scope.sha256 || ''}>
									{scope.sha256 ? scope.sha256.substring(0, 8) : '-'}
								</td>
								<td class="mono">{scope.host_usage_count}</td>
								<td>
									{scope.last_success_at ? relativeTime(scope.last_success_at) : 'Never'}
								</td>
								<td>
									{scope.next_refresh_at ? relativeTime(scope.next_refresh_at, true) : 'Never'}
								</td>
								<td class="action-cell">
									<button
										class="btn btn-secondary btn-sm sync-btn"
										onclick={() => handleSync(scope.scope_key)}
										disabled={syncingScopes[scope.scope_key]}
									>
										{#if syncingScopes[scope.scope_key]}
											<svg class="spinner" viewBox="0 0 50 50">
												<circle cx="25" cy="25" r="20"></circle>
											</svg>
											Syncing...
										{:else}
											Sync Now
										{/if}
									</button>
								</td>
							</tr>
							{#if scope.last_error}
								<tr class="error-row">
									<td colspan="9" class="error-cell">
										<span class="error-label">Error:</span>
										{scope.last_error}
									</td>
								</tr>
							{/if}
						{/each}
					</tbody>
				</table>
			{/if}
		{/if}
	</div>
</AppLayout>

<style>
	.alert-error {
		background: var(--red-bg);
		border: 1px solid var(--red-border);
		color: var(--red);
		padding: 12px 16px;
		border-radius: var(--radius-sm);
		margin-bottom: var(--space-md);
		font-size: 14px;
	}

	.empty-title {
		margin-top: 12px;
		font-weight: 600;
		color: var(--text-primary);
	}

	.empty-desc {
		margin-top: 4px;
		color: var(--text-dim);
		font-size: 13px;
	}

	.scope-key {
		font-weight: 600;
		color: var(--text-primary);
	}

	.sha-text {
		font-size: 12px;
		color: var(--text-dim);
	}

	.action-cell {
		width: 1%;
		white-space: nowrap;
		text-align: right;
	}

	.sync-btn {
		gap: 4px;
	}

	.spinner {
		display: inline-block;
		vertical-align: middle;
		width: 12px;
		height: 12px;
		animation: spin 1s linear infinite;
		stroke: currentColor;
		fill: none;
		stroke-width: 5;
		stroke-linecap: round;
	}

	.error-row {
		background: rgba(244, 63, 94, 0.03);
	}

	.error-cell {
		padding: 8px 20px;
		font-size: 12px;
		color: var(--red);
		border-top: none;
	}

	.error-label {
		font-weight: 600;
		text-transform: uppercase;
		margin-right: 8px;
	}

	@keyframes spin {
		100% {
			transform: rotate(360deg);
		}
	}
</style>