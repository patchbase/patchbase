<script lang="ts">
	import { onMount } from 'svelte';
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import ApproveHostButton from '$lib/components/ApproveHostButton.svelte';
	import DeleteHostButton from '$lib/components/DeleteHostButton.svelte';
	import {
		getHost,
		getHostSnapshot,
		listPullJobs,
		getHostVulnerablePackages,
		getHostUpgradablePackages
	} from '$lib/api/hosts.js';
	import { formatTime, formatDuration } from '$lib/format';
	import { goto } from '$app/navigation';
	import type { Host, HostSnapshot, HostPullJob, MatcherDecisionGroup } from '$lib/types';

	interface Props {
		params: { hostId: string };
	}

	let { params }: Props = $props();

	let host = $state<Host | null>(null);
	let snapshot = $state<HostSnapshot | null>(null);
	let pullJobs = $state<HostPullJob[]>([]);
	let vulnerableGroups = $state<MatcherDecisionGroup[]>([]);
	let upgradableGroups = $state<MatcherDecisionGroup[]>([]);
	let activeTab = $state<'vulnerabilities' | 'updates'>('vulnerabilities');
	let expandedGroups = $state<Record<string, boolean>>({});
	let loading = $state(true);
	let error = $state('');
	let packagesError = $state('');

	function toggleGroup(familyLabel: string) {
		expandedGroups[familyLabel] = !expandedGroups[familyLabel];
	}

	async function loadData(silent = false) {
		const id = params.hostId;
		if (!silent) {
			loading = true;
			error = '';
			packagesError = '';
		}
		try {
			const [hostData, snapshotData, jobsData, vulnsData, updatesData] = await Promise.all([
				getHost(id),
				getHostSnapshot(id).catch(() => null),
				listPullJobs(id).catch(() => [] as HostPullJob[]),
				getHostVulnerablePackages(id).catch((err) => {
					packagesError = err instanceof Error ? err.message : 'Failed to load vulnerable packages';
					return [] as MatcherDecisionGroup[];
				}),
				getHostUpgradablePackages(id).catch((err) => {
					packagesError = err instanceof Error ? err.message : 'Failed to load upgradable packages';
					return [] as MatcherDecisionGroup[];
				})
			]);

			if (JSON.stringify(host) !== JSON.stringify(hostData)) {
				host = hostData;
			}
			if (JSON.stringify(snapshot) !== JSON.stringify(snapshotData)) {
				snapshot = snapshotData;
			}
			if (JSON.stringify(pullJobs) !== JSON.stringify(jobsData)) {
				pullJobs = jobsData;
			}
			if (JSON.stringify(vulnerableGroups) !== JSON.stringify(vulnsData)) {
				vulnerableGroups = vulnsData;
			}
			if (JSON.stringify(upgradableGroups) !== JSON.stringify(updatesData)) {
				upgradableGroups = updatesData;
			}
		} catch (err) {
			if (!silent) {
				error = err instanceof Error ? err.message : 'Failed to load host details.';
			}
		} finally {
			if (!silent) {
				loading = false;
			}
		}
	}

	onMount(() => {
		void loadData();
		const interval = setInterval(() => {
			void loadData(true);
		}, 5000);
		return () => clearInterval(interval);
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
				<div class="detail-row"><span class="label">OS Family</span><span class="value">{host.os_family || '-'}</span></div>
				<div class="detail-row"><span class="label">OS Name</span><span class="value">{host.os_name || '-'}</span></div>
				<div class="detail-row"><span class="label">OS Version</span><span class="value">{host.os_version || '-'}</span></div>
				<div class="detail-row"><span class="label">Architecture</span><span class="value mono">{host.architecture || '-'}</span></div>
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

		{#if snapshot}
			<div class="tabs-container">
				<button
					class="tab-btn {activeTab === 'vulnerabilities' ? 'active' : ''}"
					onclick={() => activeTab = 'vulnerabilities'}
				>
					Vulnerabilities
					{#if vulnerableGroups.reduce((acc, g) => acc + g.package_count, 0) > 0}
						<span class="tab-badge badge badge-red">{vulnerableGroups.reduce((acc, g) => acc + g.package_count, 0)}</span>
					{/if}
				</button>
				<button
					class="tab-btn {activeTab === 'updates' ? 'active' : ''}"
					onclick={() => activeTab = 'updates'}
				>
					Updates
					{#if upgradableGroups.reduce((acc, g) => acc + g.package_count, 0) > 0}
						<span class="tab-badge badge badge-blue">{upgradableGroups.reduce((acc, g) => acc + g.package_count, 0)}</span>
					{/if}
				</button>
			</div>

			{#if packagesError}
				<div class="packages-error-banner">
					{packagesError}
				</div>
			{/if}

			{#if activeTab === 'vulnerabilities'}
				{#if vulnerableGroups.length === 0}
					<div class="empty-state-card">
						<p>No vulnerable packages found.</p>
					</div>
				{:else}
					<div class="groups-list">
						{#each vulnerableGroups as group}
							<div class="group-card">
								<div class="group-header" onclick={() => toggleGroup(group.family_label)}>
									<div class="group-title-section">
										<span class="group-toggle-icon">{expandedGroups[group.family_label] ? '▼' : '▶'}</span>
										<span class="group-title">{group.family_label}</span>
										{#if group.severity_label}
											<span class="badge badge-{group.severity_tone}">{group.severity_label}</span>
										{/if}
										{#if group.action_label}
											<span class="badge badge-{group.action_tone}">{group.action_label}</span>
										{/if}
									</div>
									<div class="group-meta">
										<span>{group.package_count} package{group.package_count > 1 ? 's' : ''}</span>
										<span>•</span>
										<span>{group.advisory_count} advisory{group.advisory_count > 1 ? 'ies' : ''}</span>
									</div>
								</div>
								{#if expandedGroups[group.family_label]}
									<div class="group-content">
										{#each group.advisories as adv}
											<div class="advisory-section">
												<div class="advisory-header">
													<div class="advisory-title-section">
														{#if adv.advisory_url}
															<a href={adv.advisory_url} target="_blank" class="advisory-link">{adv.advisory_id}</a>
														{:else}
															<span class="advisory-id">{adv.advisory_id}</span>
														{/if}
														<span class="advisory-title">{adv.title}</span>
													</div>
													{#if adv.severity_label}
														<div class="advisory-badges">
															<span class="badge badge-{adv.severity_tone}">{adv.severity_label}</span>
														</div>
													{/if}
												</div>
												<div class="advisory-packages">
													<table>
														<thead>
															<tr>
																<th>Package</th>
																<th>Installed Version</th>
																<th>Fixed Version</th>
																<th>State</th>
															</tr>
														</thead>
														<tbody>
															{#each adv.items as item}
																<tr>
																	<td class="mono font-semibold" style="color:var(--text-primary)">{item.package_name}</td>
																	<td class="mono">{item.installed_nevra}</td>
																	<td class="mono">{item.fixed_nevra}</td>
																	<td>
																		<span class="badge badge-{item.package_state_tone}">{item.package_state_label}</span>
																	</td>
																</tr>
															{/each}
														</tbody>
													</table>
												</div>
											</div>
										{/each}
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			{:else}
				{#if upgradableGroups.length === 0}
					<div class="empty-state-card">
						{#if host.available_updates > 0}
							<p>
								The host reports {host.available_updates} package update{host.available_updates === 1 ? '' : 's'},
								but this snapshot did not include per-package update details.
							</p>
						{:else}
							<p>No package updates available.</p>
						{/if}
					</div>
				{:else}
					<div class="groups-list">
						{#each upgradableGroups as group}
							<div class="group-card">
								<div class="group-header" onclick={() => toggleGroup(group.family_label)}>
									<div class="group-title-section">
										<span class="group-toggle-icon">{expandedGroups[group.family_label] ? '▼' : '▶'}</span>
										<span class="group-title">{group.family_label}</span>
										{#if group.severity_label}
											<span class="badge badge-{group.severity_tone}">{group.severity_label}</span>
										{/if}
										{#if group.action_label}
											<span class="badge badge-{group.action_tone}">{group.action_label}</span>
										{/if}
									</div>
									<div class="group-meta">
										<span>{group.package_count} package{group.package_count > 1 ? 's' : ''}</span>
										<span>•</span>
										<span>{group.advisory_count} advisory{group.advisory_count > 1 ? 'ies' : ''}</span>
									</div>
								</div>
								{#if expandedGroups[group.family_label]}
									<div class="group-content">
										{#each group.advisories as adv}
											<div class="advisory-section">
												<div class="advisory-header">
													<div class="advisory-title-section">
														{#if adv.advisory_url}
															<a href={adv.advisory_url} target="_blank" class="advisory-link">{adv.advisory_id}</a>
														{:else}
															<span class="advisory-id">{adv.advisory_id}</span>
														{/if}
														<span class="advisory-title">{adv.title}</span>
													</div>
													{#if adv.severity_label}
														<div class="advisory-badges">
															<span class="badge badge-{adv.severity_tone}">{adv.severity_label}</span>
														</div>
													{/if}
												</div>
												<div class="advisory-packages">
													<table>
														<thead>
															<tr>
																<th>Package</th>
																<th>Installed Version</th>
																<th>Fixed Version</th>
																<th>State</th>
															</tr>
														</thead>
														<tbody>
															{#each adv.items as item}
																<tr>
																	<td class="mono font-semibold" style="color:var(--text-primary)">{item.package_name}</td>
																	<td class="mono">{item.installed_nevra}</td>
																	<td class="mono">{item.fixed_nevra}</td>
																	<td>
																		<span class="badge badge-{item.package_state_tone}">{item.package_state_label}</span>
																	</td>
																</tr>
															{/each}
														</tbody>
													</table>
												</div>
											</div>
										{/each}
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			{/if}
		{/if}

		{#if host.onboarding_mode === 'ssh'}
			<div style="margin-top:24px; margin-bottom:24px">
				<h3 style="font-size:13px; font-weight:600; text-transform:uppercase; letter-spacing:0.04em; color:var(--text-dim); margin-bottom:12px">
					SSH Pull History
				</h3>
				<div class="table-container">
					{#if pullJobs.length === 0}
						<div style="padding: 24px; text-align: center; color: var(--text-dim);">
							No pull jobs recorded yet.
						</div>
					{:else}
						<table>
							<thead>
								<tr>
									<th>Job ID</th>
									<th>Status</th>
									<th>Started At</th>
									<th>Duration</th>
									<th>Error</th>
								</tr>
							</thead>
							<tbody>
								{#each pullJobs as job}
									<tr>
										<td class="mono">{job.id}</td>
										<td><StatusBadge status={job.status} /></td>
										<td>{formatTime(job.started_at)}</td>
										<td>{formatDuration(job.started_at, job.completed_at)}</td>
										<td style="max-width:300px; text-overflow:ellipsis; overflow:hidden; white-space:nowrap" title={job.error || ''}>
											{job.error || '-'}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					{/if}
				</div>
			</div>
		{/if}
	</AppLayout>
{:else}
	<AppLayout page="hosts" title="Host">
		<div class="empty-state"><p>Host not found.</p></div>
	</AppLayout>
{/if}

<style>
	.tabs-container {
		display: flex;
		gap: 8px;
		border-bottom: 1px solid var(--border);
		margin-top: 24px;
		margin-bottom: 16px;
	}

	.tab-btn {
		background: transparent;
		border: none;
		border-bottom: 2px solid transparent;
		color: var(--text-secondary);
		padding: 10px 16px;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.15s;
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.tab-btn:hover {
		color: var(--text-primary);
	}

	.tab-btn.active {
		color: var(--accent);
		border-bottom-color: var(--accent);
	}

	.tab-badge {
		font-size: 11px;
		padding: 2px 6px;
	}

	.empty-state-card {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 32px;
		text-align: center;
		color: var(--text-dim);
	}

	.groups-list {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.group-card {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		overflow: hidden;
	}

	.group-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 16px 20px;
		cursor: pointer;
		user-select: none;
		transition: background 0.1s;
	}

	.group-header:hover {
		background: var(--bg-card-hover);
	}

	.group-title-section {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.group-toggle-icon {
		font-size: 10px;
		color: var(--text-dim);
		width: 12px;
	}

	.group-title {
		font-size: 15px;
		font-weight: 700;
		color: var(--text-primary);
	}

	.group-meta {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		color: var(--text-dim);
	}

	.group-content {
		border-top: 1px solid var(--border);
		background: var(--bg-secondary);
		padding: 16px 20px;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.advisory-section {
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--bg-card);
		overflow: hidden;
	}

	.advisory-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 12px 16px;
		background: var(--bg-card-hover);
		border-bottom: 1px solid var(--border);
	}

	.advisory-title-section {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.advisory-link {
		font-family: var(--font-mono);
		font-size: 13px;
		font-weight: 700;
		color: var(--accent);
		text-decoration: underline;
	}

	.advisory-id {
		font-family: var(--font-mono);
		font-size: 13px;
		font-weight: 700;
		color: var(--text-primary);
	}

	.advisory-title {
		font-size: 14px;
		color: var(--text-secondary);
	}

	.advisory-packages {
		overflow-x: auto;
	}

	.advisory-packages table {
		width: 100%;
		border-collapse: collapse;
	}

	.advisory-packages th {
		background: transparent;
		padding: 8px 16px;
		font-size: 11px;
		color: var(--text-dim);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		border-bottom: 1px solid var(--border);
	}

	.advisory-packages td {
		padding: 10px 16px;
		font-size: 13px;
		border-bottom: 1px solid var(--border);
	}

	.advisory-packages tr:last-child td {
		border-bottom: none;
	}

	.font-semibold {
		font-weight: 600;
	}

	.packages-error-banner {
		background: rgba(248, 113, 113, 0.1);
		border: 1px solid var(--red);
		border-radius: 8px;
		color: var(--red);
		padding: 12px 16px;
		font-size: 14px;
		font-weight: 500;
		margin-bottom: 16px;
	}
</style>
