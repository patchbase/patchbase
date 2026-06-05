<script lang="ts">
	import { onMount } from 'svelte';
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import ApproveHostButton from '$lib/components/ApproveHostButton.svelte';
	import DeleteHostButton from '$lib/components/DeleteHostButton.svelte';
	import {
		getHost,
		getHostKernelPosture,
		getHostSnapshot,
		listPullJobs,
		runPullNow,
		getHostVulnerablePackages,
		getHostUpgradablePackages,
		ingestManualReport
	} from '$lib/api/hosts.js';
	import { formatTime, formatDuration } from '$lib/format';
	import { goto } from '$app/navigation';
	import type { Host, HostKernelPosture, HostSnapshot, HostPullJob, MatcherDecisionGroup } from '$lib/types';

	interface Props {
		params: { hostId: string };
	}

	let { params }: Props = $props();

	let host = $state<Host | null>(null);
	let snapshot = $state<HostSnapshot | null>(null);
	let pullJobs = $state<HostPullJob[]>([]);
	let vulnerableGroups = $state<MatcherDecisionGroup[]>([]);
	let upgradableGroups = $state<MatcherDecisionGroup[]>([]);
	let kernelPosture = $state<HostKernelPosture | null>(null);
	let activeTab = $state<'vulnerabilities' | 'updates' | 'kernel'>('vulnerabilities');
	let expandedGroups = $state<Record<string, boolean>>({});
	let loading = $state(true);
	let error = $state('');
	let packagesError = $state('');
	let kernelError = $state('');

	let manualFileContent = $state('');
	let manualFileName = $state('');
	let uploading = $state(false);
	let uploadError = $state('');
	let uploadSuccess = $state(false);
	let fileInput = $state<HTMLInputElement | null>(null);
	let runningPullNow = $state(false);

	function handleFileChange(event: Event) {
		const input = event.target as HTMLInputElement;
		if (input.files && input.files[0]) {
			const file = input.files[0];
			manualFileName = file.name;
			const reader = new FileReader();
			reader.onload = (e) => {
				manualFileContent = e.target?.result as string || '';
			};
			reader.readAsText(file);
		} else {
			manualFileContent = '';
			manualFileName = '';
		}
	}

	async function submitReport() {
		if (!manualFileContent || !host) {
			uploadError = 'Please select a report file first.';
			return;
		}
		uploading = true;
		uploadError = '';
		uploadSuccess = false;
		try {
			await ingestManualReport(host.id, manualFileContent);
			uploadSuccess = true;
			manualFileContent = '';
			manualFileName = '';
			if (fileInput) {
				fileInput.value = '';
			}
			await loadData(true);
		} catch (err) {
			uploadError = err instanceof Error ? err.message : 'Failed to upload report.';
		} finally {
			uploading = false;
		}
	}

	async function runPullNowJob() {
		if (!host || runningPullNow) {
			return;
		}

		runningPullNow = true;
		try {
			await runPullNow(host.id);
			await loadData(true);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to run SSH pull now.';
		} finally {
			runningPullNow = false;
		}
	}

	function toggleGroup(familyLabel: string) {
		expandedGroups[familyLabel] = !expandedGroups[familyLabel];
	}

	function hasCveScore(score: number | null | undefined): score is number {
		return score !== undefined && score !== null;
	}

	function getCveStyle(score: number | null | undefined) {
		if (!hasCveScore(score)) return '';
		if (score >= 7.0) return 'background-color: #dc2626; color: #ffffff; border-color: #dc2626;';
		if (score >= 4.0) return 'background-color: #ea580c; color: #ffffff; border-color: #ea580c;';
		if (score > 0) return 'background-color: #ca8a04; color: #ffffff; border-color: #ca8a04;';
		return '';
	}


	function sortCvesByScore(cves: CVEInfo[] | null | undefined): CVEInfo[] {
		if (!cves) return [];
		return [...cves].sort((a, b) => {
			const sa = a.score ?? -1;
			const sb = b.score ?? -1;
			return sb - sa;
		});
	}

	function compactKernelValue(value: string | null | undefined) {
		const trimmed = (value || '').trim();
		if (!trimmed) {
			return '-';
		}

		// APT latest kernel values are often full package identifiers like:
		// linux-image-6.8.0-117-generic-0:6.8.0-117.117.amd64
		if (trimmed.startsWith('linux-image-')) {
			let normalized = trimmed.replace(/^linux-image-(unsigned-)?/, '');
			const epochMatch = /-\d+:/.exec(normalized);
			if (epochMatch && epochMatch.index > 0) {
				normalized = normalized.slice(0, epochMatch.index);
			}
			return normalized || trimmed;
		}

		// RPM latest kernel values are often full NEVRA-like strings:
		// kernel-0:6.12.0-124.56.1.el10_1.x86_64
		const rpmMatch = /^[A-Za-z0-9._+-]+-\d+:(.+)$/.exec(trimmed);
		if (rpmMatch && rpmMatch[1]) {
			return rpmMatch[1];
		}

		return trimmed;
	}

	async function loadData(silent = false) {
		const id = params.hostId;
		if (!silent) {
			loading = true;
			error = '';
			packagesError = '';
			kernelError = '';
		}
		try {
			const [hostData, snapshotData, jobsData, vulnsData, updatesData, kernelData] = await Promise.all([
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
				}),
				getHostKernelPosture(id).catch((err) => {
					kernelError = err instanceof Error ? err.message : 'Failed to load kernel posture';
					return null;
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
			if (JSON.stringify(kernelPosture) !== JSON.stringify(kernelData)) {
				kernelPosture = kernelData;
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

		{#if host.onboarding_mode === 'manual'}
			<div class="detail-card" style="margin-bottom:24px">
				<h3>Upload New Report</h3>
				<p class="form-hint" style="margin-bottom: 16px; color: var(--text-secondary); font-size: 13px; max-width: 600px;">
					Run the collector script on the host to gather package and system data, then select the generated report text file to update this host's status.
				</p>
				
				<div class="upload-container" style="display: flex; flex-direction: column; gap: 12px; max-width: 500px;">
					{#if uploadError}
						<div class="upload-error" style="color: var(--red); font-size: 13px; background: var(--red-bg); padding: 8px 12px; border-radius: 6px; border: 1px solid var(--red);">
							{uploadError}
						</div>
					{/if}
					
					{#if uploadSuccess}
						<div class="upload-success" style="color: var(--green); font-size: 13px; background: var(--green-bg); padding: 8px 12px; border-radius: 6px; border: 1px solid var(--green);">
							Report uploaded and processed successfully!
						</div>
					{/if}

					<div style="display: flex; gap: 12px; align-items: center; flex-wrap: wrap;">
						<input
							bind:this={fileInput}
							type="file"
							accept=".txt,.sh,.log"
							onchange={handleFileChange}
							class="form-input"
							style="flex: 1; min-width: 200px; padding: 6px 12px; border: 1px solid var(--border); border-radius: 6px; background: var(--bg-secondary); color: var(--text-primary); cursor: pointer;"
						/>
						<button
							class="btn btn-primary btn-sm"
							type="button"
							onclick={submitReport}
							disabled={!manualFileContent || uploading}
							style="min-width: 120px;"
						>
							{#if uploading}
								Uploading...
							{:else}
								Upload Report
							{/if}
						</button>
					</div>
				</div>
			</div>
		{/if}

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
				<button
					class="tab-btn {activeTab === 'kernel' ? 'active' : ''}"
					onclick={() => activeTab = 'kernel'}
				>
					Kernel
					{#if kernelPosture && kernelPosture.active_kernel.cve_count > 0}
						<span class="tab-badge badge badge-warn">{kernelPosture.active_kernel.cve_count}</span>
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
														{#if adv.cves && adv.cves.length > 0}
															<span class="advisory-cves" style="display: inline-flex; gap: 4px; margin-left: 8px; flex-wrap: wrap; align-items: center; vertical-align: middle;">
																{#each sortCvesByScore(adv.cves) as cve}
																	<a href={cve.url || `https://nvd.nist.gov/vuln/detail/${cve.id}`} target="_blank" rel="noopener noreferrer" class="badge badge-secondary" style="font-size: 11px; text-decoration: none; padding: 2px 6px; {getCveStyle(cve.score)}" title={hasCveScore(cve.score) ? `CVSS Score: ${cve.score}` : 'No CVSS score available'}>{cve.id}{hasCveScore(cve.score) ? ` (${cve.score})` : ''}</a>
																{/each}
															</span>
														{/if}
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
																		{#if item.reason_text}
																			<div style="font-size: 11px; color: var(--text-dim); margin-top: 4px; max-width: 280px; line-height: 1.3;">{item.reason_text}</div>
																		{/if}
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
			{:else if activeTab === 'updates'}
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
														{#if adv.cves && adv.cves.length > 0}
															<span class="advisory-cves" style="display: inline-flex; gap: 4px; margin-left: 8px; flex-wrap: wrap; align-items: center; vertical-align: middle;">
																{#each sortCvesByScore(adv.cves) as cve}
																	<a href={cve.url || `https://nvd.nist.gov/vuln/detail/${cve.id}`} target="_blank" rel="noopener noreferrer" class="badge badge-secondary" style="font-size: 11px; text-decoration: none; padding: 2px 6px; {getCveStyle(cve.score)}" title={hasCveScore(cve.score) ? `CVSS Score: ${cve.score}` : 'No CVSS score available'}>{cve.id}{hasCveScore(cve.score) ? ` (${cve.score})` : ''}</a>
																{/each}
															</span>
														{/if}
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
																		{#if item.reason_text}
																			<div style="font-size: 11px; color: var(--text-dim); margin-top: 4px; max-width: 280px; line-height: 1.3;">{item.reason_text}</div>
																		{/if}
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
				<div class="detail-card">
					<h3>Kernel Vulnerability Posture</h3>
					{#if kernelError}
						<div class="packages-error-banner" style="margin-top:12px">{kernelError}</div>
					{:else if kernelPosture}
						<div class="detail-row"><span class="label">Running Kernel</span><span class="value mono">{compactKernelValue(kernelPosture.running_kernel)}</span></div>
						<div class="detail-row"><span class="label">Latest Installed Kernel</span><span class="value mono" title={kernelPosture.latest_installed_kernel || ''}>{compactKernelValue(kernelPosture.latest_installed_kernel)}</span></div>
						<div class="detail-row"><span class="label">Reboot Impact</span><span class="value">{kernelPosture.reboot_would_reduce_cve_count ? 'Reboot would reduce active kernel CVEs' : 'No CVE reduction expected from reboot'}</span></div>

						<div style="margin-top:12px; display:grid; grid-template-columns:repeat(auto-fit, minmax(260px, 1fr)); gap:12px;">
							<div style="border:1px solid var(--border); border-radius:10px; padding:12px;">
								<div style="font-weight:600; margin-bottom:8px;">Active Kernel</div>
								<div class="detail-row"><span class="label">Advisories</span><span class="value">{kernelPosture.active_kernel.advisory_count}</span></div>
								<div class="detail-row"><span class="label">CVEs</span><span class="value">{kernelPosture.active_kernel.cve_count}</span></div>
								<div class="detail-row"><span class="label">Packages</span><span class="value">{kernelPosture.active_kernel.package_count}</span></div>
							</div>
							<div style="border:1px solid var(--border); border-radius:10px; padding:12px;">
								<div style="font-weight:600; margin-bottom:8px;">Latest Installed Kernel</div>
								<div class="detail-row"><span class="label">Advisories</span><span class="value">{kernelPosture.latest_installed.advisory_count}</span></div>
								<div class="detail-row"><span class="label">CVEs</span><span class="value">{kernelPosture.latest_installed.cve_count}</span></div>
								<div class="detail-row"><span class="label">Packages</span><span class="value">{kernelPosture.latest_installed.package_count}</span></div>
							</div>
						</div>

						{#if kernelPosture.active_kernel.advisories.length > 0}
							<div style="margin-top:16px;">
								<h4 style="margin-bottom:8px;">Active Kernel Advisories</h4>
								<div class="groups-list">
									{#each kernelPosture.active_kernel.advisories as advisory}
										<div class="group-card">
											<div class="group-header">
												<div class="group-title-section">
													{#if advisory.advisory_url}
														<a href={advisory.advisory_url} target="_blank" class="advisory-link">{advisory.advisory_id}</a>
													{:else}
														<span class="group-title">{advisory.advisory_id}</span>
													{/if}
													<span class="badge badge-{advisory.severity_tone}">{advisory.severity_label}</span>
													<span class="badge badge-{advisory.action_tone}">{advisory.action_label}</span>
												</div>
											</div>
											<div class="group-content">
												<div class="advisory-title">{advisory.title}</div>
												{#if advisory.cves && advisory.cves.length > 0}
													<div class="advisory-cves" style="display:flex; gap:6px; margin-top:8px; flex-wrap:wrap;">
														{#each sortCvesByScore(advisory.cves) as cve}
															<a href={cve.url || `https://nvd.nist.gov/vuln/detail/${cve.id}`} target="_blank" rel="noopener noreferrer" class="badge badge-secondary" style="font-size:11px; text-decoration:none; padding:2px 6px; {getCveStyle(cve.score)}" title={hasCveScore(cve.score) ? `CVSS Score: ${cve.score}` : 'No CVSS score available'}>{cve.id}{hasCveScore(cve.score) ? ` (${cve.score})` : ''}</a>
														{/each}
													</div>
												{/if}
											</div>
										</div>
									{/each}
								</div>
							</div>
						{/if}
						{#if kernelPosture.latest_installed.advisories.length > 0}
							<div style="margin-top:16px;">
								<h4 style="margin-bottom:8px;">Latest Installed Kernel Advisories</h4>
								<div class="groups-list">
									{#each kernelPosture.latest_installed.advisories as advisory}
										<div class="group-card">
											<div class="group-header">
												<div class="group-title-section">
													{#if advisory.advisory_url}
														<a href={advisory.advisory_url} target="_blank" class="advisory-link">{advisory.advisory_id}</a>
													{:else}
														<span class="group-title">{advisory.advisory_id}</span>
													{/if}
													<span class="badge badge-{advisory.severity_tone}">{advisory.severity_label}</span>
													<span class="badge badge-{advisory.action_tone}">{advisory.action_label}</span>
												</div>
											</div>
											<div class="group-content">
												<div class="advisory-title">{advisory.title}</div>
												{#if advisory.cves && advisory.cves.length > 0}
													<div class="advisory-cves" style="display:flex; gap:6px; margin-top:8px; flex-wrap:wrap;">
														{#each sortCvesByScore(advisory.cves) as cve}
															<a href={cve.url || `https://nvd.nist.gov/vuln/detail/${cve.id}`} target="_blank" rel="noopener noreferrer" class="badge badge-secondary" style="font-size:11px; text-decoration:none; padding:2px 6px; {getCveStyle(cve.score)}" title={hasCveScore(cve.score) ? `CVSS Score: ${cve.score}` : 'No CVSS score available'}>{cve.id}{hasCveScore(cve.score) ? ` (${cve.score})` : ''}</a>
														{/each}
													</div>
												{/if}
											</div>
										</div>
									{/each}
								</div>
							</div>
						{/if}
					{:else}
						<div class="empty-state-card" style="margin-top:12px;">
							<p>No kernel posture data available.</p>
						</div>
					{/if}
				</div>
			{/if}
		{/if}

			{#if host.onboarding_mode === 'ssh'}
				<div style="margin-top:24px; margin-bottom:24px">
					<div style="display:flex; align-items:center; justify-content:space-between; gap:12px; margin-bottom:12px;">
						<h3 style="font-size:13px; font-weight:600; text-transform:uppercase; letter-spacing:0.04em; color:var(--text-dim); margin:0">
							SSH Pull History
						</h3>
						<button class="btn btn-secondary btn-sm" type="button" onclick={runPullNowJob} disabled={runningPullNow}>
							{runningPullNow ? 'Running...' : 'Run Now'}
						</button>
					</div>
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
