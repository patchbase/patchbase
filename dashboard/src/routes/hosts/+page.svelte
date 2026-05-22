<script lang="ts">
	import { onMount } from 'svelte';
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatsRow from '$lib/components/StatsRow.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import ApproveHostButton from '$lib/components/ApproveHostButton.svelte';
	import DeleteHostButton from '$lib/components/DeleteHostButton.svelte';
	import { listHosts } from '$lib/api/hosts.js';
	import { relativeTime } from '$lib/format';
	import type { Host } from '$lib/types';

	type ViewMode = 'grid' | 'list';
	const viewModeStorageKey = 'patchbase_hosts_view_mode';

	let hosts = $state<Host[]>([]);
	let loading = $state(true);
	let error = $state('');
	let viewMode = $state<ViewMode>('grid');
	let openActionsHostID = $state<string | null>(null);

	async function loadHosts(silent = false): Promise<void> {
		if (!silent) {
			loading = true;
			error = '';
		}
		try {
			const newHosts = await listHosts();
			if (JSON.stringify(hosts) !== JSON.stringify(newHosts)) {
				hosts = newHosts;
			}
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
		void loadHosts();

		const interval = setInterval(() => {
			void loadHosts(true);
		}, 5000);

		const onDocumentClick = (event: MouseEvent): void => {
			const target = event.target as HTMLElement | null;
			if (target?.closest('.host-actions')) {
				return;
			}
			openActionsHostID = null;
		};

		document.addEventListener('click', onDocumentClick);
		return () => {
			clearInterval(interval);
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

	let needAttention = $derived(hosts.filter((h) => h.overall_action !== 'none').length);
	let rebootQueue = $derived(hosts.filter((h) => h.needs_reboot > 0).length);
	let totalUpdates = $derived(hosts.reduce((sum, h) => sum + h.available_updates, 0));

	let stats = $derived([
		{ label: 'Total Hosts', value: hosts.length, color: 'accent' },
		{ label: 'Need Attention', value: needAttention, color: 'red' },
		{ label: 'Reboot Queue', value: rebootQueue, color: 'orange' },
		{ label: 'Pending Updates', value: totalUpdates, color: 'blue' },
	]);

	const actionOrder: Record<string, number> = {
		reboot_host: 0,
		restart_service: 1,
		update_package: 2,
		investigate: 3,
		none: 4,
	};

	let sortedHosts = $derived(
		[...hosts].sort((a, b) => (actionOrder[a.overall_action] ?? 5) - (actionOrder[b.overall_action] ?? 5)),
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
												hosts = hosts.filter((item) => item.id !== host.id);
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
						<StatusBadge status={host.approval_status || 'unknown'} />
						{#if host.critical_count > 0}
							<span class="badge badge-red"><span class="badge-dot"></span>{host.critical_count} critical</span>
						{/if}
						{#if host.important_count > 0}
							<span class="badge badge-orange"><span class="badge-dot"></span>{host.important_count} important</span>
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
					<th>Host</th>
					<th>Approval</th>
					<th>Platform</th>
					<th>Action</th>
					<th>Critical</th>
					<th>Updates</th>
					<th>Reboot</th>
					<th>Last Check</th>
					<th></th>
				</tr>
			</thead>
			<tbody>
				{#each sortedHosts as host (host.id)}
					<tr>
						<td><a href="/hosts/{host.id}">{hostLabel(host)}</a></td>
						<td><StatusBadge status={host.approval_status || 'unknown'} /></td>
						<td>{host.os_name} {host.os_version} ({host.architecture})</td>
						<td><StatusBadge status={host.overall_action} /></td>
						<td class="mono">{host.critical_count}</td>
						<td class="mono">{host.available_updates}</td>
						<td class="mono">{host.needs_reboot}</td>
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
										<DeleteHostButton
											{host}
											class="host-actions-item host-actions-item-danger"
											onDelete={() => {
												hosts = hosts.filter((item) => item.id !== host.id);
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
