// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
<script lang="ts">
	interface Stat {
		label: string;
		value: string | number;
		color: string;
	}

	interface Props {
		stats: Stat[];
	}

	let { stats }: Props = $props();
</script>

<div class="stats-row">
	{#each stats as stat}
		<div class="stat-card {stat.color}">
			<div class="stat-label">{stat.label}</div>
			<div class="stat-value">{stat.value}</div>
		</div>
	{/each}
</div>