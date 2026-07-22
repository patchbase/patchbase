<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
	import { onMount } from 'svelte';
	import AppLayout from '$lib/components/AppLayout.svelte';
	import { listAuditEntries, type AuditLogEntry } from '$lib/api/audit.js';
	import { relativeTime } from '$lib/format';

	const pageSize = 25;

	let loading = $state(true);
	let error = $state('');
	let entries = $state<AuditLogEntry[]>([]);
	let total = $state(0);
	let offset = $state(0);

	let actionFilter = $state('');
	let actorFilter = $state('');
	let fromFilter = $state('');
	let toFilter = $state('');

	let expandedIds = $state<Record<string, boolean>>({});

	const totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)));
	const currentPage = $derived(Math.floor(offset / pageSize) + 1);

	const actionLabels: Record<string, string> = {
		'auth.login.success': 'Login success',
		'auth.login.failure': 'Login failure',
		'auth.profile.update': 'Profile update',
		'host.create': 'Host created',
		'host.delete': 'Host deleted',
		'host.approve': 'Host approved',
		'host.registration_token.create': 'Token created',
		'host.registration_token.revoke': 'Token revoked',
		'host.ssh.pull': 'SSH pull',
		'host.ssh.onboard': 'SSH host onboarded',
		'host.manual.ingest': 'Manual report ingested',
		'settings.update': 'Setting updated',
	};

	function describeAction(action: string): string {
		return actionLabels[action] ?? action;
	}

	function buildParams(): {
		limit: number;
		offset: number;
		action?: string;
		actor?: string;
		from?: string;
		to?: string;
	} {
		const params: ReturnType<typeof buildParams> = {
			limit: pageSize,
			offset,
		};
		const action = actionFilter.trim();
		const actor = actorFilter.trim();
		if (action) params.action = action;
		if (actor) params.actor = actor;
		if (fromFilter) params.from = fromFilter;
		if (toFilter) params.to = toFilter;
		return params;
	}

	async function loadEntries(): Promise<void> {
		loading = true;
		error = '';
		try {
			const response = await listAuditEntries(buildParams());
			entries = response.items;
			total = response.total;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load audit log.';
		} finally {
			loading = false;
		}
	}

	function goToPage(target: number): void {
		const next = Math.min(Math.max(1, target), totalPages);
		offset = (next - 1) * pageSize;
		void loadEntries();
	}

	function toggleExpanded(id: string): void {
		expandedIds = { ...expandedIds, [id]: !expandedIds[id] };
	}

	function formatMetadata(entry: AuditLogEntry): string {
		if (!entry.metadata) return '';
		return JSON.stringify(entry.metadata, null, 2);
	}

	function clearFilters(): void {
		actionFilter = '';
		actorFilter = '';
		fromFilter = '';
		toFilter = '';
		offset = 0;
		void loadEntries();
	}

	onMount(() => {
		void loadEntries();
	});
</script>

<AppLayout page="audit-log" title="Audit Log">
	{#if error}
		<div style="background:var(--red-bg); border:1px solid var(--red); color:var(--red); padding:12px 16px; border-radius:8px; margin-bottom:20px; font-size:14px;">
			{error}
		</div>
	{/if}

	<div class="filters">
		<label class="filter-label" for="audit-action-filter">Action</label>
		<input
			id="audit-action-filter"
			type="text"
			class="form-input"
			bind:value={actionFilter}
			placeholder="e.g. host.create"
		/>
		<label class="filter-label" for="audit-actor-filter">Actor</label>
		<input
			id="audit-actor-filter"
			type="text"
			class="form-input"
			bind:value={actorFilter}
			placeholder="e.g. u_admin"
		/>
		<label class="filter-label" for="audit-from-filter">From</label>
		<input
			id="audit-from-filter"
			type="date"
			class="form-input"
			bind:value={fromFilter}
		/>
		<label class="filter-label" for="audit-to-filter">To</label>
		<input
			id="audit-to-filter"
			type="date"
			class="form-input"
			bind:value={toFilter}
		/>
		<button
			type="button"
			class="btn btn-secondary btn-sm"
			onclick={() => {
				offset = 0;
				void loadEntries();
			}}
		>
			Apply
		</button>
		<button type="button" class="btn btn-secondary btn-sm" onclick={clearFilters}>
			Clear
		</button>
	</div>

	<div class="table-container">
		<div class="table-header">
			<h2>Activity</h2>
			<span class="muted">Showing {entries.length} of {total} events</span>
		</div>

		{#if loading}
			<div class="empty-state"><p>Loading audit log…</p></div>
		{:else if entries.length === 0}
			<div class="empty-state">
				<p style="margin-top:12px;font-weight:600;color:var(--text-primary)">No audit events</p>
				<p style="margin-top:4px;color:var(--text-dim);font-size:13px">
					Activity will appear here as administrators perform actions.
				</p>
			</div>
		{:else}
			<table>
				<thead>
					<tr>
						<th>When</th>
						<th>Actor</th>
						<th>Action</th>
						<th>Target</th>
						<th>IP</th>
						<th>Details</th>
					</tr>
				</thead>
				<tbody>
					{#each entries as entry (entry.id)}
						{@const metadata = formatMetadata(entry)}
						<tr>
							<td title={entry.created_at}>{relativeTime(entry.created_at)}</td>
							<td>
								<div class="actor">
									<span class="actor-email">{entry.actor_email}</span>
									{#if entry.actor_id}
										<span class="actor-id muted">{entry.actor_id}</span>
									{/if}
								</div>
							</td>
							<td>
								<span class="action-tag">{describeAction(entry.action)}</span>
								<div class="muted action-raw">{entry.action}</div>
							</td>
							<td>
								<div class="target">
									<span class="target-type muted">{entry.target_type}</span>
									{#if entry.target_id}
										<span class="target-id">{entry.target_id}</span>
									{:else}
										<span class="muted">—</span>
									{/if}
								</div>
							</td>
							<td>
								{#if entry.ip_address}
									<span class="mono">{entry.ip_address}</span>
								{:else}
									<span class="muted">—</span>
								{/if}
							</td>
							<td>
								{#if metadata}
									<button
										type="button"
										class="btn btn-secondary btn-sm"
										onclick={() => toggleExpanded(entry.id)}
									>
										{expandedIds[entry.id] ? 'Hide' : 'Show'}
									</button>
								{:else}
									<span class="muted">—</span>
								{/if}
							</td>
						</tr>
						{#if expandedIds[entry.id] && metadata}
							<tr class="metadata-row">
								<td colspan="6">
									<pre class="metadata-block">{metadata}</pre>
									{#if entry.user_agent}
										<div class="muted user-agent">User-Agent: {entry.user_agent}</div>
									{/if}
								</td>
							</tr>
						{/if}
					{/each}
				</tbody>
			</table>
		{/if}
	</div>

	{#if !loading && total > pageSize}
		<div class="pagination">
			<button
				type="button"
				class="btn btn-secondary btn-sm"
				disabled={currentPage <= 1}
				onclick={() => goToPage(currentPage - 1)}
			>
				Previous
			</button>
			<span class="muted">Page {currentPage} of {totalPages}</span>
			<button
				type="button"
				class="btn btn-secondary btn-sm"
				disabled={currentPage >= totalPages}
				onclick={() => goToPage(currentPage + 1)}
			>
				Next
			</button>
		</div>
	{/if}
</AppLayout>

<style>
	.filters {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-bottom: 16px;
		flex-wrap: wrap;
	}
	.filter-label {
		font-size: 13px;
		font-weight: 500;
		color: var(--text-secondary);
	}
	.filters .form-input {
		max-width: 220px;
	}
	.table-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}
	.table-header .muted {
		font-size: 13px;
	}
	.actor {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.actor-email {
		font-weight: 500;
		color: var(--text-primary);
	}
	.actor-id {
		font-size: 12px;
		font-family: var(--font-mono);
	}
	.action-tag {
		display: inline-block;
		padding: 2px 8px;
		background: var(--bg-secondary);
		border-radius: 4px;
		font-weight: 500;
		color: var(--text-primary);
	}
	.action-raw {
		font-size: 11px;
		font-family: var(--font-mono);
		margin-top: 2px;
	}
	.target {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.target-type {
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.target-id {
		font-family: var(--font-mono);
		font-size: 13px;
	}
	.mono {
		font-family: var(--font-mono);
		font-size: 13px;
	}
	.metadata-row td {
		background: var(--bg-secondary);
		padding: 12px 16px;
		border-top: none;
	}
	.metadata-block {
		margin: 0;
		font-family: var(--font-mono);
		font-size: 12px;
		white-space: pre-wrap;
		word-break: break-word;
		color: var(--text-primary);
	}
	.user-agent {
		margin-top: 8px;
		font-size: 12px;
	}
	.pagination {
		display: flex;
		justify-content: flex-end;
		align-items: center;
		gap: 12px;
		margin-top: 16px;
	}
</style>