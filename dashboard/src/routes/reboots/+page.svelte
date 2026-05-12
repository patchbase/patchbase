<script lang="ts">
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatsRow from '$lib/components/StatsRow.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { api } from '$lib/mocks/api.js';
	import { formatTime } from '$lib/format';
	import type { Host } from '$lib/types';

	let hosts = $state<Host[]>([]);

	$effect(() => {
		api.listHosts().then((data) => (hosts = data));
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
						<StatusBadge status="reboot_host" />
					</div>
					<div class="host-card-meta">
						{host.os_name} {host.os_version} &middot; {host.architecture}
					</div>
					<div class="host-card-signals">
						<span class="badge badge-red">
							<span class="badge-dot"></span>
							{host.needs_reboot} reboot{host.needs_reboot !== 1 ? 's' : ''}
						</span>
						{#if host.critical_count > 0}
							<span class="badge badge-red">{host.critical_count} critical</span>
						{/if}
					</div>
					<div class="host-card-footer">
						<span><StatusBadge status={host.status} /></span>
						<span>Updated {formatTime(host.updated_at)}</span>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</AppLayout>