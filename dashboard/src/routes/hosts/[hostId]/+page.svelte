<script lang="ts">
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import ApproveHostButton from '$lib/components/ApproveHostButton.svelte';
	import DeleteHostButton from '$lib/components/DeleteHostButton.svelte';
	import { getHost, getHostSnapshot } from '$lib/api/hosts.js';
	import { formatTime } from '$lib/format';
	import { goto } from '$app/navigation';
	import type { Host, HostSnapshot } from '$lib/types';

	interface Props {
		params: { hostId: string };
	}

	let { params }: Props = $props();

	let host = $state<Host | null>(null);
	let snapshot = $state<HostSnapshot | null>(null);
	let loading = $state(true);
	let error = $state('');

	$effect(() => {
		const id = params.hostId;
		loading = true;
		error = '';
		Promise.all([
			getHost(id),
			getHostSnapshot(id).catch(() => null)
		])
			.then(([hostData, snapshotData]) => {
				host = hostData;
				snapshot = snapshotData;
			})
			.catch((err) => {
				error = err instanceof Error ? err.message : 'Failed to load host details.';
			})
			.finally(() => {
				loading = false;
			});
	});
</script>

{#snippet pageActions()}
	{#if host}
		<div style="display:flex;align-items:center;gap:8px">
			<ApproveHostButton
				{host}
				class="btn btn-secondary btn-sm"
				onApprove={() => {
					const id = params.hostId;
					Promise.all([
						getHost(id),
						getHostSnapshot(id).catch(() => null)
					]).then(([hostData, snapshotData]) => {
						host = hostData;
						snapshot = snapshotData;
					}).catch((err) => {
						error = err instanceof Error ? err.message : 'Failed to reload host.';
					});
				}}
				onError={(err) => {
					error = err.message;
				}}
			/>
			<DeleteHostButton
				{host}
				class="btn btn-danger btn-sm"
				buttonText="Delete Host"
				onDelete={() => {
					void goto('/hosts');
				}}
				onError={(err) => {
					error = err.message;
				}}
			/>
		</div>
	{/if}
{/snippet}

{#if loading}
	<AppLayout page="hosts" title="Host">
		<div class="empty-state"><p>Loading...</p></div>
	</AppLayout>
{:else if error}
	<AppLayout page="hosts" title="Host">
		<div class="empty-state"><p>{error}</p></div>
	</AppLayout>
{:else if host}
	<AppLayout page="hosts" title={host.display_name || host.hostname || host.id} actions={pageActions}>
		<div class="detail-grid">
			<div class="detail-card">
				<h3>Host Info</h3>
				<div class="detail-row"><span class="label">Host ID</span><span class="value mono">{host.id}</span></div>
				<div class="detail-row"><span class="label">Hostname</span><span class="value">{host.hostname || '-'}</span></div>
				<div class="detail-row"><span class="label">IP</span><span class="value mono">{host.ip_address || '-'}</span></div>
				<div class="detail-row"><span class="label">Mode</span><span class="value"><StatusBadge status={host.onboarding_mode || 'unknown'} /></span></div>
				<div class="detail-row"><span class="label">Approval</span><span class="value"><StatusBadge status={host.approval_status || 'unknown'} /></span></div>
				<div class="detail-row"><span class="label">Host Status</span><span class="value"><StatusBadge status={host.status} /></span></div>
				<div class="detail-row"><span class="label">Action</span><span class="value"><StatusBadge status={host.overall_action} /></span></div>
				<div class="detail-row"><span class="label">Last Seen</span><span class="value">{formatTime(host.last_seen_at)}</span></div>
				<div class="detail-row"><span class="label">Last Advisory Check</span><span class="value">{formatTime(host.last_advisory_check_at)}</span></div>
			</div>

			{#if snapshot}
				<div class="detail-card">
					<h3>Security Posture</h3>
					<div class="detail-row"><span class="label">Available Updates</span><span class="value">{host.available_updates}</span></div>
					<div class="detail-row"><span class="label">Critical</span><span class="value" style="color:var(--red)">{host.critical_count}</span></div>
					<div class="detail-row"><span class="label">Important</span><span class="value" style="color:var(--orange)">{host.important_count}</span></div>
					<div class="detail-row"><span class="label">Moderate</span><span class="value" style="color:var(--yellow)">{host.moderate_count}</span></div>
					<div class="detail-row"><span class="label">Needs Reboot</span><span class="value">{host.needs_reboot}</span></div>
					<div class="detail-row"><span class="label">Needs Restart</span><span class="value">{host.needs_restart}</span></div>
					<div class="detail-row"><span class="label">No Fix</span><span class="value">{host.no_fix}</span></div>
				</div>
			{/if}
		</div>

		{#if snapshot}
			<div class="detail-card" style="margin-bottom:24px">
				<h3>Latest Snapshot</h3>
				<div class="detail-row"><span class="label">Snapshot ID</span><span class="value mono">{snapshot.id}</span></div>
				<div class="detail-row"><span class="label">Running Kernel</span><span class="value">{snapshot.running_kernel_nevra || '-'}</span></div>
				<div class="detail-row"><span class="label">Booted</span><span class="value">{formatTime(snapshot.boot_time)}</span></div>
				<div class="detail-row"><span class="label">Collected</span><span class="value">{formatTime(snapshot.collected_at)}</span></div>
				<div class="detail-row"><span class="label">Process Data</span><span class="value">{snapshot.has_process_data ? 'Yes' : 'No'}</span></div>
			</div>
		{:else}
			<div class="detail-card" style="margin-bottom:24px; padding: 32px; text-align: center;">
				<p style="color: var(--text-muted); margin: 0;">No snapshot processed yet.</p>
			</div>
		{/if}
	</AppLayout>
{:else}
	<AppLayout page="hosts" title="Host">
		<div class="empty-state"><p>Host not found.</p></div>
	</AppLayout>
{/if}
