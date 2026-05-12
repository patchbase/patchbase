<script lang="ts">
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatsRow from '$lib/components/StatsRow.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { api } from '$lib/mocks/api.js';
	import { relativeTime } from '$lib/format';
	import type { Host } from '$lib/types';

	let hosts = $state<Host[]>([]);

	$effect(() => {
		api.listHosts().then((data) => (hosts = data));
	});

	let needAttention = $derived(hosts.filter((h) => h.overall_action !== 'none').length);
	let rebootQueue = $derived(hosts.filter((h) => h.needs_reboot > 0).length);
	let investigate = $derived(hosts.filter((h) => h.overall_action === 'investigate').length);
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

<AppLayout page="hosts" title="Host Fleet">
	<StatsRow {stats} />
	<div class="host-grid">
		{#each sortedHosts as host (host.id)}
			<div class="host-card">
				<div class="host-card-header">
					<div class="host-card-name">
						<a href="/hosts/{host.id}">{host.display_name || host.hostname}</a>
					</div>
					<StatusBadge status={host.overall_action} />
				</div>
				<div class="host-card-meta">
					{host.os_name} {host.os_version} &middot; {host.architecture}
				</div>
				<div class="host-card-signals">
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
					{#if host.needs_restart > 0}
						<span class="badge badge-orange">{host.needs_restart} restart</span>
					{/if}
					{#if host.no_fix > 0}
						<span class="badge badge-purple">{host.no_fix} no fix</span>
					{/if}
				</div>
				<div class="host-card-footer">
					<span><StatusBadge status={host.status} /></span>
					<span>{relativeTime(host.last_seen_at)}</span>
				</div>
			</div>
		{/each}
	</div>
</AppLayout>