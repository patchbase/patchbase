<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
	import AppLayout from '$lib/components/AppLayout.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	let advisory = $derived(data.advisory);
</script>

<AppLayout page="advisories" title="Advisory Detail">
	<div class="header" style="margin-bottom: 24px;">
		<a href="/advisories" class="btn btn-secondary btn-sm" style="margin-bottom: 16px; display: inline-flex;">&larr; Back to Advisories</a>
		<h1 style="margin: 0; font-size: 24px;">{advisory.raw_source_id}</h1>
		{#if advisory.summary}
			<p style="margin-top: 8px; color: var(--text-secondary); font-size: 16px;">{advisory.summary}</p>
		{/if}
	</div>

	<div class="detail-card">
		<div class="detail-grid">
			<div class="detail-item">
				<span class="label">ID</span>
				<span class="value mono">{advisory.id}</span>
			</div>
			<div class="detail-item">
				<span class="label">Vendor</span>
				<span class="value">{advisory.vendor}</span>
			</div>
			<div class="detail-item">
				<span class="label">Type</span>
				<span class="value" style="text-transform: capitalize;">{advisory.advisory_type}</span>
			</div>
			<div class="detail-item">
				<span class="label">Severity</span>
				<span class="value">
					{#if advisory.severity}
						<span class="badge badge-{advisory.severity.toLowerCase()}">{advisory.severity}</span>
					{:else}
						<span class="badge">Unknown</span>
					{/if}
				</span>
			</div>
			<div class="detail-item">
				<span class="label">Published</span>
				<span class="value">{advisory.published_at ? new Date(advisory.published_at).toLocaleDateString() : 'Unknown'}</span>
			</div>
			{#if advisory.source_url}
				<div class="detail-item">
					<span class="label">Source URL</span>
					<a href={advisory.source_url} target="_blank" rel="noreferrer" class="value link">
						View upstream advisory ↗
					</a>
				</div>
			{/if}
		</div>

		{#if advisory.description}
			<div class="description-section">
				<h3>Description</h3>
				<div class="description-content">
					{advisory.description}
				</div>
			</div>
		{/if}
	</div>
</AppLayout>

<style>
	.detail-card {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 24px;
	}

	.detail-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
		gap: 24px;
		margin-bottom: 32px;
	}

	.detail-item {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.label {
		font-size: 13px;
		color: var(--text-dim);
		font-weight: 500;
		text-transform: uppercase;
		letter-spacing: 0.5px;
	}

	.value {
		font-size: 15px;
		color: var(--text-primary);
	}

	.link {
		color: var(--accent);
		text-decoration: none;
	}
	
	.link:hover {
		text-decoration: underline;
	}

	.description-section {
		border-top: 1px solid var(--border);
		padding-top: 24px;
	}

	.description-section h3 {
		margin: 0 0 16px 0;
		font-size: 16px;
		color: var(--text-primary);
	}

	.description-content {
		color: var(--text-secondary);
		font-size: 14px;
		line-height: 1.6;
		white-space: pre-wrap;
	}
</style>
