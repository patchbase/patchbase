<script lang="ts">
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatsRow from '$lib/components/StatsRow.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { getDashboardOverview } from '$lib/api/dashboard.js';
	import { listHosts } from '$lib/api/hosts.js';
	import { relativeTime } from '$lib/format';
	import type { Host } from '$lib/types';

	let overview = $state<Awaited<ReturnType<typeof getDashboardOverview>> | null>(null);
	let recentHosts = $state<Host[]>([]);

	$effect(() => {
		getDashboardOverview().then((data) => (overview = data));
		listHosts().then((data) => {
			recentHosts = data.sort((a, b) => {
				if ((b.critical_count || 0) !== (a.critical_count || 0)) {
					return (b.critical_count || 0) - (a.critical_count || 0);
				}
				if ((b.important_count || 0) !== (a.important_count || 0)) {
					return (b.important_count || 0) - (a.important_count || 0);
				}
				if ((b.moderate_count || 0) !== (a.moderate_count || 0)) {
					return (b.moderate_count || 0) - (a.moderate_count || 0);
				}
				const actionOrder: Record<string, number> = {
					reboot_host: 0,
					restart_service: 1,
					update_package: 2,
					investigate: 3,
					none: 4,
				};
				const aOrder = actionOrder[a.overall_action] ?? 5;
				const bOrder = actionOrder[b.overall_action] ?? 5;
				return aOrder - bOrder;
			}).slice(0, 5);
		});
	});

	let stats = $derived(
		overview
			? [
					{ label: 'Total Hosts', value: overview.total_hosts, color: 'accent' },
					{ label: 'Need Attention', value: overview.need_attention, color: 'red' },
					{ label: 'Reboot Queue', value: overview.reboot_queue, color: 'orange' },
					{ label: 'Advisories', value: overview.total_advisories, color: 'purple' },
				]
			: [],
	);
</script>

<AppLayout page="dashboard" title="Dashboard">
	<StatsRow {stats} />
	<div class="table-container" style="margin-bottom:24px">
		<div class="table-header">
			<h2>Host Fleet</h2>
			<a href="/hosts" class="btn btn-secondary btn-sm">View all</a>
		</div>
		<table>
			<thead>
				<tr>
					<th>Host</th>
					<th>Platform</th>
					<th>Action</th>
					<th>Critical</th>
					<th>Updates</th>
					<th>Last Seen</th>
				</tr>
			</thead>
			<tbody>
				{#each recentHosts as host}
					<tr>
						<td class="mono">
							<a href="/hosts/{host.id}">{host.display_name || host.hostname}</a>
						</td>
						<td>{host.os_name} {host.os_version}</td>
						<td><StatusBadge status={host.overall_action} /></td>
						<td class="mono">{host.critical_count}</td>
						<td class="mono">{host.available_updates}</td>
						<td>{relativeTime(host.last_seen_at)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
	<div class="table-container">
		<div class="table-header">
			<h2>Recent Advisories</h2>
			<a href="/advisories" class="btn btn-secondary btn-sm">View all</a>
		</div>
		{#if overview && overview.recent_advisories?.length}
			<table>
				<thead>
					<tr>
						<th>ID</th>
						<th>Severity</th>
						<th>Summary</th>
						<th>Published</th>
					</tr>
				</thead>
				<tbody>
					{#each overview.recent_advisories as adv}
						<tr>
							<td class="mono">
								<a href="/advisories/{adv.id}">{adv.id}</a>
							</td>
							<td>
								{#if adv.severity}
									<span class="badge badge-{adv.severity.toLowerCase()}">{adv.severity}</span>
								{:else}
									<span class="badge">Unknown</span>
								{/if}
							</td>
							<td>{adv.summary || 'No summary'}</td>
							<td>{adv.published_at ? new Date(adv.published_at).toLocaleDateString() : 'Unknown'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
		{#if overview}
			<p style="padding:20px;color:var(--text-secondary)">
				Tracking {overview.total_advisories} advisories across {overview.total_scopes} advisory scopes.
			</p>
		{/if}
	</div>
</AppLayout>