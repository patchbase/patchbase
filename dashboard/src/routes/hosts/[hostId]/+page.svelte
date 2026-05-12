<script lang="ts">
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { api } from '$lib/mocks/api.js';
	import { formatTime } from '$lib/format';
	import type { Host, HostSnapshot, DecisionGroup } from '$lib/types';

	interface Props {
		params: { hostId: string };
	}

	let { params }: Props = $props();

	let host = $state<Host | null>(null);
	let snapshot = $state<HostSnapshot | null>(null);
	let decisionGroups = $state<DecisionGroup[]>([]);
	let expandedGroups = $state<Set<string>>(new Set());

	$effect(() => {
		const id = params.hostId;
		api.getHost(id).then((data) => (host = data));
		api.getHostSnapshot(id).then((data) => (snapshot = data));
		api.getDecisionGroups(id).then((data) => (decisionGroups = data));
	});

	function toggleGroup(family: string) {
		expandedGroups.update((set) => {
			const next = new Set(set);
			if (next.has(family)) {
				next.delete(family);
			} else {
				next.add(family);
			}
			return next;
		});
	}

	let totalPackages = $derived(decisionGroups.reduce((sum, g) => sum + g.package_count, 0));
	let totalAdvisories = $derived(decisionGroups.reduce((sum, g) => sum + g.advisory_count, 0));
</script>

{#if host}
	<AppLayout page="hosts" title={host.display_name || host.hostname}>
		<div class="detail-grid">
			<div class="detail-card">
				<h3>Host Info</h3>
				<div class="detail-row">
					<span class="label">Hostname</span>
					<span class="value">{host.hostname}</span>
				</div>
				<div class="detail-row">
					<span class="label">Platform</span>
					<span class="value">{host.os_name} {host.os_version}</span>
				</div>
				<div class="detail-row">
					<span class="label">Architecture</span>
					<span class="value">{host.architecture}</span>
				</div>
				<div class="detail-row">
					<span class="label">Status</span>
					<span class="value"><StatusBadge status={host.status} /></span>
				</div>
				<div class="detail-row">
					<span class="label">Action</span>
					<span class="value"><StatusBadge status={host.overall_action} /></span>
				</div>
				<div class="detail-row">
					<span class="label">Last Seen</span>
					<span class="value">{formatTime(host.last_seen_at)}</span>
				</div>
				<div class="detail-row">
					<span class="label">State Updated</span>
					<span class="value">{formatTime(host.updated_at)}</span>
				</div>
			</div>
			<div class="detail-card">
				<h3>Security Posture</h3>
				<div class="detail-row">
					<span class="label">Available Updates</span>
					<span class="value">{host.available_updates}</span>
				</div>
				<div class="detail-row">
					<span class="label">Critical</span>
					<span class="value" style="color:var(--red)">{host.critical_count}</span>
				</div>
				<div class="detail-row">
					<span class="label">Important</span>
					<span class="value" style="color:var(--orange)">{host.important_count}</span>
				</div>
				<div class="detail-row">
					<span class="label">Moderate</span>
					<span class="value" style="color:var(--yellow)">{host.moderate_count}</span>
				</div>
				<div class="detail-row">
					<span class="label">Needs Reboot</span>
					<span class="value">{host.needs_reboot}</span>
				</div>
				<div class="detail-row">
					<span class="label">Needs Restart</span>
					<span class="value">{host.needs_restart}</span>
				</div>
				<div class="detail-row">
					<span class="label">No Fix Available</span>
					<span class="value">{host.no_fix}</span>
				</div>
			</div>
		</div>

		{#if snapshot}
			<div class="detail-card" style="margin-bottom:24px">
				<h3>Latest Snapshot</h3>
				<div class="detail-row">
					<span class="label">Running Kernel</span>
					<span class="value">{snapshot.running_kernel_nevra}</span>
				</div>
				<div class="detail-row">
					<span class="label">Booted</span>
					<span class="value">{formatTime(snapshot.boot_time)}</span>
				</div>
				<div class="detail-row">
					<span class="label">Collected</span>
					<span class="value">{formatTime(snapshot.collected_at)}</span>
				</div>
				<div class="detail-row">
					<span class="label">Process Data</span>
					<span class="value">{snapshot.has_process_data ? 'Yes' : 'No'}</span>
				</div>
			</div>
		{/if}

		<h2 style="font-size:16px;font-weight:700;margin-bottom:16px">
			Decision Groups ({totalPackages} packages, {totalAdvisories} advisories)
		</h2>

		{#if decisionGroups.length === 0}
			<div class="empty-state">
				<p>No active security actions for this host.</p>
			</div>
		{:else}
			{#each decisionGroups as group (group.family)}
				<div class="decision-group">
					<button type="button" class="decision-group-header" onclick={() => toggleGroup(group.family)} style="width:100%;text-align:left;background:none;border:none;cursor:pointer;color:inherit;font:inherit;padding:14px 20px">
						<div style="display:flex;align-items:center;gap:12px">
							<h3 style="margin:0">
								{group.family}
							</h3>
							<StatusBadge status={group.severity} />
							<StatusBadge status={group.action} />
							<span style="font-size:12px;color:var(--text-dim)">
								{group.advisory_count} advisories &middot; {group.package_count} packages
							</span>
						</div>
						<span style="color:var(--text-dim);font-size:12px">{expandedGroups.has(group.family) ? '▼' : '▶'}</span>
				</button>
					{#if expandedGroups.has(group.family)}
						<div class="decision-group-body">
							<table>
								<thead>
									<tr>
										<th>Package</th>
										<th>Installed</th>
										<th>Fixed</th>
										<th>Status</th>
										<th>Action</th>
										<th>Reason</th>
									</tr>
								</thead>
								<tbody>
									{#each group.decisions as d}
										<tr>
											<td class="mono">{d.package_name}</td>
											<td class="mono" style="font-size:12px">{d.installed_nevra || '-'}</td>
											<td class="mono" style="font-size:12px">
												{#if d.fixed_nevra}
													<span style="color:var(--green)">{d.fixed_nevra}</span>
												{:else}
													-
												{/if}
											</td>
											<td><StatusBadge status={d.status} /></td>
											<td><StatusBadge status={d.action} /></td>
											<td style="color:var(--text-secondary);font-size:13px">{d.reason_text}</td>
										</tr>
									{/each}
								</tbody>
							</table>
							{#each group.decisions as d}
								{#if d.advisory_raw_id !== group.decisions[0].advisory_raw_id || d === group.decisions[0]}
									<div style="margin-top:12px;padding:8px 0;border-top:1px solid var(--border)">
										<div style="display:flex;align-items:center;gap:8px;margin-bottom:4px">
											{#if d.advisory_raw_id.startsWith('RHSA') || d.advisory_raw_id.startsWith('RLSA')}
												<a href="/advisories" class="mono" style="font-size:13px;font-weight:600">{d.advisory_raw_id}</a>
											{:else}
												<span class="mono" style="font-size:13px;font-weight:600">{d.advisory_raw_id}</span>
											{/if}
											<StatusBadge status={d.advisory_severity} />
										</div>
										<div style="font-size:13px;color:var(--text-secondary)">{d.advisory_summary}</div>
									</div>
								{/if}
							{/each}
						</div>
					{/if}
				</div>
			{/each}
		{/if}
	</AppLayout>
{:else}
	<AppLayout page="hosts" title="Host">
		<div class="empty-state">
			<p>Loading...</p>
		</div>
	</AppLayout>
{/if}