<script lang="ts">
	import AppLayout from '$lib/components/AppLayout.svelte';
	import StatsRow from '$lib/components/StatsRow.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { api } from '$lib/mocks/api.js';
	import { relativeTime } from '$lib/format';
	import type { Advisory, ProductStream, SourceSync } from '$lib/types';

	let advisories = $state<Advisory[]>([]);
	let streams = $state<ProductStream[]>([]);
	let sources = $state<SourceSync[]>([]);
	let selectedStream = $state<string>('');

	$effect(() => {
		api.listAdvisories().then((data) => (advisories = data));
		api.listProductStreams().then((data) => {
			streams = data;
			if (!selectedStream && data.length > 0) {
				selectedStream = data[0].id;
			}
		});
		api.listSourceSyncs().then((data) => (sources = data));
	});

	let filteredAdvisories = $derived(
		selectedStream
			? advisories.filter((a) => a.product_streams.includes(selectedStream))
			: advisories,
	);

	let totalAdvisories = $derived(advisories.length);
	let totalStreams = $derived(streams.length);
	let syncedSources = $derived(sources.filter((s) => s.status === 'synced').length);

	let stats = $derived([
		{ label: 'Advisories', value: totalAdvisories, color: 'accent' },
		{ label: 'Product Streams', value: totalStreams, color: 'blue' },
		{ label: 'Synced Sources', value: syncedSources, color: 'green' },
		{ label: 'Security', value: advisories.filter((a) => a.is_security).length, color: 'red' },
	]);
</script>

<AppLayout page="advisories" title="Advisories">
	<StatsRow {stats} />

	<h2 style="font-size:15px;font-weight:700;margin-bottom:12px">Source Sync Status</h2>
	<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:12px;margin-bottom:24px">
		{#each sources as source}
			<div class="detail-card" style="margin-bottom:0">
				<h3>{source.name}</h3>
				<div class="detail-row">
					<span class="label">Advisories</span>
					<span class="value">{source.advisory_count}</span>
				</div>
				<div class="detail-row">
					<span class="label">Streams</span>
					<span class="value">{source.stream_count}</span>
				</div>
				<div class="detail-row">
					<span class="label">Last Sync</span>
					<span class="value">{source.last_sync ? relativeTime(source.last_sync) : 'Never'}</span>
				</div>
				<div class="detail-row">
					<span class="label">Status</span>
					<span class="value"><StatusBadge status={source.status} /></span>
				</div>
			</div>
		{/each}
	</div>

	<h2 style="font-size:15px;font-weight:700;margin-bottom:12px">Product Streams</h2>
	<div style="display:flex;flex-wrap:wrap;gap:8px;margin-bottom:24px">
		{#each streams as stream}
			<button
				class="btn {selectedStream === stream.id ? 'btn-primary' : 'btn-secondary'} btn-sm"
				onclick={() => (selectedStream = stream.id)}
			>
				{stream.distro_name} {stream.major_version} {stream.repo_family} {stream.architecture}
				<span style="opacity:0.7;margin-left:4px">({stream.advisory_count})</span>
			</button>
		{/each}
	</div>

	<div class="table-container">
		<div class="table-header">
			<h2>Advisories ({filteredAdvisories.length})</h2>
		</div>
		{#if filteredAdvisories.length === 0}
			<div class="empty-state">
				<p>No advisories for this stream.</p>
			</div>
		{:else}
			{#each filteredAdvisories as advisory}
				<div class="advisory-card">
					<div class="advisory-card-kicker">
						{advisory.source_system.replace(/_/g, ' ')} &middot; {advisory.advisory_type}
					</div>
					<div class="advisory-card-header">
						<div style="display:flex;align-items:center;gap:8px">
							{#if advisory.source_url}
								<a href={advisory.source_url} target="_blank" rel="noopener" class="advisory-card-title">
									{advisory.raw_source_id}
								</a>
							{:else}
								<span class="advisory-card-title">{advisory.raw_source_id}</span>
							{/if}
							<StatusBadge status={advisory.severity} />
							<span class="cap-tag">{advisory.evidence_tier}</span>
						</div>
						<span style="font-size:12px;color:var(--text-dim)">{advisory.package_count} packages</span>
					</div>
					<div class="advisory-card-description">{advisory.summary}</div>
					<div class="advisory-card-meta">
						<span>{relativeTime(advisory.published_at)}</span>
						<span>&middot;</span>
						<span>{advisory.vendor}</span>
						<span>&middot;</span>
						<span>{advisory.product_streams.length} streams</span>
					</div>
				</div>
			{/each}
		{/if}
	</div>
</AppLayout>