// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
<script lang="ts">
	import { onMount } from 'svelte';
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatsRow from '$lib/components/StatsRow.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import ApproveHostButton from '$lib/components/ApproveHostButton.svelte';
	import DeleteHostButton from '$lib/components/DeleteHostButton.svelte';
	import { listHosts, runPullNow } from '$lib/api/hosts.js';
	import { relativeTime } from '$lib/format';
	import type { Host } from '$lib/types';

	import { createPollingFallback } from '$lib/ws/fallback';
	import { hosts, hostsConnected } from '$lib/stores/hosts';

	type ViewMode = 'grid' | 'list';
	const viewModeStorageKey = 'patchbase_hosts_view_mode';
	let loading = $state(true);
	let error = $state('');
	let viewMode = $state<ViewMode>('grid');
	let openActionsHostID = $state<string | null>(null);
	let runningPullNowHostId = $state<string | null>(null);

	async function runPullNowJob(hostID: string): Promise<void> {
		if (runningPullNowHostId) return;
		runningPullNowHostId = hostID;
		error = '';
		try {
			await runPullNow(hostID);
			void loadHosts(true);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to run SSH pull now.';
		} finally {
			runningPullNowHostId = null;
			openActionsHostID = null;
		}
	}

	async function loadHosts(silent = false): Promise<void> {
		if (!silent) {
			loading = true;
			error = '';
		}
		try {
			const newHosts = await listHosts();
			// Since we use the store now, we can update it directly
			hosts.set(newHosts);
		} catch (err) {
			if (!silent) {
				error = err instanceof Error ? err.message : 'Failed to load hosts.';
			} else {
				console.error('Failed to poll hosts:', err);
			}
		} finally {
			if (!silent) {
				loading = false;
			}
		}
	}

	onMount(() => {
		const saved = window.localStorage.getItem(viewModeStorageKey);
		if (saved === 'grid' || saved === 'list') {
			viewMode = saved;
		}

		if ($hosts.length === 0) {
			void loadHosts();
		} else {
			loading = false;
		}

		let fallbackTimer: ReturnType<typeof setTimeout> | undefined;
		const fallback = createPollingFallback(() => loadHosts(true), 5000);
		const unsubConnected = hostsConnected.subscribe((connected) => {
			if (!connected) {
				fallbackTimer = setTimeout(() => { fallback.start(); }, 10000);
			} else {
				clearTimeout(fallbackTimer);
				fallback.stop();
			}
		});

		const onDocumentClick = (event: MouseEvent): void => {
			const target = event.target as HTMLElement | null;
			if (target?.closest('.host-actions')) {
				return;
			}
			openActionsHostID = null;
		};

		document.addEventListener('click', onDocumentClick);
		return () => {
			unsubConnected();
			clearTimeout(fallbackTimer);
			fallback.stop();
			document.removeEventListener('click', onDocumentClick);
		};
	});

	function setViewMode(mode: ViewMode): void {
		viewMode = mode;
		window.localStorage.setItem(viewModeStorageKey, mode);
		openActionsHostID = null;
	}

	function toggleHostActions(hostID: string): void {
		openActionsHostID = openActionsHostID === hostID ? null : hostID;
	}

	function hostLabel(host: Host): string {
		return host.display_name || host.hostname || host.id;
	}

	let needAttention = $derived($hosts.filter((h) => h.overall_action !== 'none').length);
	let rebootQueue = $derived($hosts.filter((h) => h.needs_reboot > 0).length);
	let totalUpdates = $derived($hosts.reduce((sum, h) => sum + h.available_updates, 0));

	let stats = $derived([
		{ label: 'Total Hosts', value: $hosts.length, color: 'accent' },
		{ label: 'Need Attention', value: needAttention, color: 'red' },
		{ label: 'Reboot Queue', value: rebootQueue, color: 'orange' },
		{ label: 'Pending Updates', value: totalUpdates, color: 'blue' },
	]);

	type SortField = 'host' | 'platform' | 'critical' | 'important' | 'moderate' | 'updates' | 'last_check';
	type SortDirection = 'asc' | 'desc';

	let sortField = $state<SortField | null>(null);
	let sortDirection = $state<SortDirection>('asc');

	function handleSort(field: SortField): void {
		if (sortField === field) {
			sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
		} else {
			sortField = field;
			if (field === 'host' || field === 'platform') {
				sortDirection = 'asc';
			} else {
				sortDirection = 'desc';
			}
		}
	}

	const actionOrder: Record<string, number> = {
		reboot_host: 0,
		restart_service: 1,
		update_package: 2,
		investigate: 3,
		none: 4,
	};

	let sortedHosts = $derived(
		[...$hosts].sort((a, b) => {
			if (!sortField) {
				if ((b.critical_count || 0) !== (a.critical_count || 0)) {
					return (b.critical_count || 0) - (a.critical_count || 0);
				}
				if ((b.important_count || 0) !== (a.important_count || 0)) {
					return (b.important_count || 0) - (a.important_count || 0);
				}
				if ((b.moderate_count || 0) !== (a.moderate_count || 0)) {
					return (b.moderate_count || 0) - (a.moderate_count || 0);
				}
				const aOrder = actionOrder[a.overall_action] ?? 5;
				const bOrder = actionOrder[b.overall_action] ?? 5;
				return aOrder - bOrder;
			}

			if (sortField === 'host') {
				const labelA = hostLabel(a).toLowerCase();
				const labelB = hostLabel(b).toLowerCase();
				if (labelA < labelB) return sortDirection === 'asc' ? -1 : 1;
				if (labelA > labelB) return sortDirection === 'asc' ? 1 : -1;
				return 0;
			}

			if (sortField === 'platform') {
				const platA = `${a.os_name} ${a.os_version} ${a.architecture}`.toLowerCase();
				const platB = `${b.os_name} ${b.os_version} ${b.architecture}`.toLowerCase();
				if (platA < platB) return sortDirection === 'asc' ? -1 : 1;
				if (platA > platB) return sortDirection === 'asc' ? 1 : -1;
				return 0;
			}

			if (sortField === 'critical') {
				const valA = a.critical_count || 0;
				const valB = b.critical_count || 0;
				return sortDirection === 'asc' ? valA - valB : valB - valA;
			}

			if (sortField === 'important') {
				const valA = a.important_count || 0;
				const valB = b.important_count || 0;
				return sortDirection === 'asc' ? valA - valB : valB - valA;
			}

			if (sortField === 'moderate') {
				const valA = a.moderate_count || 0;
				const valB = b.moderate_count || 0;
				return sortDirection === 'asc' ? valA - valB : valB - valA;
			}

			if (sortField === 'updates') {
				const valA = a.available_updates || 0;
				const valB = b.available_updates || 0;
				return sortDirection === 'asc' ? valA - valB : valB - valA;
			}

			if (sortField === 'last_check') {
				const timeA = a.last_advisory_check_at ? new Date(a.last_advisory_check_at).getTime() : 0;
				const timeB = b.last_advisory_check_at ? new Date(b.last_advisory_check_at).getTime() : 0;
				return sortDirection === 'asc' ? timeA - timeB : timeB - timeA;
			}

			return 0;
		}),
	);
</script>

{#snippet pageActions()}
	<div style="display:flex;align-items:center;gap:8px">
		<a class="btn btn-secondary btn-sm" href="/hosts/register">Register Host</a>
		<div class="segmented-toggle" role="group" aria-label="Hosts view mode">
			<button type="button" class:active={viewMode === 'grid'} onclick={() => setViewMode('grid')}>Grid</button>
			<button type="button" class:active={viewMode === 'list'} onclick={() => setViewMode('list')}>List</button>
		</div>
	</div>
{/snippet}

<AppLayout page="hosts" title="Host Fleet" actions={pageActions}>
	<StatsRow {stats} />

	{#if loading}
		<div class="empty-state"><p>Loading hosts...</p></div>
	{:else if error}
		<div class="empty-state">
			<p>{error}</p>
			<button class="btn btn-secondary btn-sm" type="button" onclick={() => void loadHosts()}>Retry</button>
		</div>
	{:else if sortedHosts.length === 0}
		<div class="empty-state"><p>No hosts registered yet.</p></div>
	{:else if viewMode === 'grid'}
		<div class="host-grid">
			{#each sortedHosts as host (host.id)}
				<div class="host-card">
					<div class="host-card-header">
						<div class="host-card-name">
							<a href="/hosts/{host.id}">{hostLabel(host)}</a>
						</div>
						<div class="host-card-header-right">
							<StatusBadge status={host.overall_action} />
							<div class="host-actions">
								<button
									type="button"
									class="host-actions-trigger"
									aria-label={`Host actions for ${hostLabel(host)}`}
									aria-expanded={openActionsHostID === host.id}
									onclick={() => toggleHostActions(host.id)}
								>
									<span></span>
									<span></span>
									<span></span>
								</button>
								{#if openActionsHostID === host.id}
									<div class="host-actions-menu">
										<ApproveHostButton
											{host}
											class="host-actions-item"
											onApprove={() => {
												openActionsHostID = null;
												void loadHosts();
											}}
											onError={(err) => {
												openActionsHostID = null;
												error = err.message;
											}}
										/>
										<DeleteHostButton
											{host}
											class="host-actions-item host-actions-item-danger"
											onDelete={() => {
												hosts.update(items => items.filter((item) => item.id !== host.id));
												openActionsHostID = null;
											}}
											onError={(err) => {
												error = err.message;
												openActionsHostID = null;
											}}
										/>
									</div>
								{/if}
							</div>
						</div>
					</div>
					<div class="host-card-meta">
						{host.os_name} {host.os_version} &middot; {host.architecture}
					</div>
					<div class="host-card-signals">
						{#if host.approval_status === 'waiting_approval'}
							<StatusBadge status="waiting_approval" />
						{:else if host.approval_status === 'rejected'}
							<StatusBadge status="rejected" />
						{/if}
						{#if host.critical_count > 0}
							<span class="badge badge-red"><span class="badge-dot"></span>{host.critical_count} critical</span>
						{/if}
						{#if host.important_count > 0}
							<span class="badge badge-orange"><span class="badge-dot"></span>{host.important_count} important</span>
						{/if}
						{#if host.moderate_count > 0}
							<span class="badge badge-yellow"><span class="badge-dot"></span>{host.moderate_count} moderate</span>
						{/if}
						{#if host.available_updates > 0}
							<span class="badge badge-blue">{host.available_updates} updates</span>
						{/if}
						{#if host.needs_reboot > 0}
							<span class="badge badge-red">{host.needs_reboot} reboot</span>
						{/if}
					</div>
					<div class="host-card-footer">
						<span>{relativeTime(host.last_seen_at)}</span>
						<span>{relativeTime(host.last_advisory_check_at || null)}</span>
					</div>
				</div>
			{/each}
		</div>
	{:else}
		<table>
			<thead>
				<tr>
					<th onclick={() => handleSort('host')} class="sortable-header">
						Host
						{#if sortField === 'host'}
							<span class="sort-direction">{sortDirection === 'asc' ? '▲' : '▼'}</span>
						{/if}
					</th>
					<th onclick={() => handleSort('platform')} class="sortable-header">
						Platform
						{#if sortField === 'platform'}
							<span class="sort-direction">{sortDirection === 'asc' ? '▲' : '▼'}</span>
						{/if}
					</th>
					<th>Action</th>
					<th onclick={() => handleSort('critical')} class="sortable-header">
						Critical
						{#if sortField === 'critical'}
							<span class="sort-direction">{sortDirection === 'asc' ? '▲' : '▼'}</span>
						{/if}
					</th>
					<th onclick={() => handleSort('important')} class="sortable-header">
						Important
						{#if sortField === 'important'}
							<span class="sort-direction">{sortDirection === 'asc' ? '▲' : '▼'}</span>
						{/if}
					</th>
					<th onclick={() => handleSort('moderate')} class="sortable-header">
						Moderate
						{#if sortField === 'moderate'}
							<span class="sort-direction">{sortDirection === 'asc' ? '▲' : '▼'}</span>
						{/if}
					</th>
					<th onclick={() => handleSort('updates')} class="sortable-header">
						Updates
						{#if sortField === 'updates'}
							<span class="sort-direction">{sortDirection === 'asc' ? '▲' : '▼'}</span>
						{/if}
					</th>
					<th onclick={() => handleSort('last_check')} class="sortable-header">
						Last Check
						{#if sortField === 'last_check'}
							<span class="sort-direction">{sortDirection === 'asc' ? '▲' : '▼'}</span>
						{/if}
					</th>
					<th></th>
				</tr>
			</thead>
			<tbody>
				{#each sortedHosts as host (host.id)}
					<tr>
						<td><a href="/hosts/{host.id}">{hostLabel(host)}</a></td>
						<td>{host.os_name} {host.os_version} ({host.architecture})</td>
						<td>
							{#if host.approval_status === 'waiting_approval'}
								<StatusBadge status="waiting_approval" />
							{:else if host.approval_status === 'rejected'}
								<StatusBadge status="rejected" />
							{:else}
								<StatusBadge status={host.overall_action} />
							{/if}
						</td>
						<td class="mono">{host.critical_count || '-'}</td>
						<td class="mono">{host.important_count || '-'}</td>
						<td class="mono">{host.moderate_count || '-'}</td>
						<td class="mono">{host.available_updates || '-'}</td>
						<td>{relativeTime(host.last_advisory_check_at || null)}</td>
						<td class="host-actions-cell">
							<div class="host-actions">
								<button
									type="button"
									class="host-actions-trigger"
									aria-label={`Host actions for ${hostLabel(host)}`}
									aria-expanded={openActionsHostID === host.id}
									onclick={() => toggleHostActions(host.id)}
								>
									<span></span>
									<span></span>
									<span></span>
								</button>
								{#if openActionsHostID === host.id}
									<div class="host-actions-menu host-actions-menu-right">
										<ApproveHostButton
											{host}
											class="host-actions-item"
											onApprove={() => {
												openActionsHostID = null;
												void loadHosts();
											}}
											onError={(err) => {
												openActionsHostID = null;
												error = err.message;
											}}
										/>
										{#if host.onboarding_mode === 'ssh'}
											<button
												type="button"
												class="host-actions-item"
												onclick={() => void runPullNowJob(host.id)}
												disabled={runningPullNowHostId !== null}
											>
												{runningPullNowHostId === host.id ? 'Running...' : 'Run now'}
											</button>
										{/if}
										<DeleteHostButton
											{host}
											class="host-actions-item host-actions-item-danger"
											onDelete={() => {
												hosts.update(items => items.filter((item) => item.id !== host.id));
												openActionsHostID = null;
											}}
											onError={(err) => {
												error = err.message;
												openActionsHostID = null;
											}}
										/>
									</div>
								{/if}
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</AppLayout>

<style>
	.sortable-header {
		cursor: pointer;
		user-select: none;
		transition: background-color 0.15s ease, color 0.15s ease;
	}
	.sortable-header:hover {
		background: var(--bg-card-hover);
		color: var(--text-primary);
	}
	.sort-direction {
		font-size: 10px;
		margin-left: 4px;
		color: var(--accent);
		display: inline-block;
		vertical-align: middle;
	}
</style>
