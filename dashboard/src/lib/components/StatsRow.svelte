<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
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

<style>
	.stats-row {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: var(--space-md);
		margin-bottom: var(--space-lg);
	}

	.stat-card {
		position: relative;
		background: var(--bg-card);
		backdrop-filter: blur(12px);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 20px;
		overflow: hidden;
		transition: transform var(--transition-normal), border-color var(--transition-normal), box-shadow var(--transition-normal);

		&::before {
			content: '';
			position: absolute;
			top: 0;
			left: 0;
			right: 0;
			height: 3px;
			background: var(--text-dim);
			opacity: 0.8;
			transition: opacity var(--transition-fast);
		}

		&:hover {
			transform: translateY(-2px);
			border-color: var(--border-light);
			box-shadow: var(--shadow-md);
		}
	}

	.stat-label {
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-dim);
		margin-bottom: var(--space-xs);
	}

	.stat-value {
		font-size: 28px;
		font-weight: 800;
		letter-spacing: -0.02em;
		color: var(--text-primary);
	}

	.stat-card.accent {
		&::before { background: var(--accent); }
		.stat-value { color: var(--accent-light); }
	}
	.stat-card.green {
		&::before { background: var(--green); }
		.stat-value { color: var(--green); }
	}
	.stat-card.blue {
		&::before { background: var(--blue); }
		.stat-value { color: var(--blue); }
	}
	.stat-card.purple {
		&::before { background: var(--purple); }
		.stat-value { color: var(--purple); }
	}
	.stat-card.red {
		&::before { background: var(--red); }
		.stat-value { color: var(--red); }
	}
	.stat-card.orange {
		&::before { background: var(--orange); }
		.stat-value { color: var(--orange); }
	}
	.stat-card.yellow {
		&::before { background: var(--yellow); }
		.stat-value { color: var(--yellow); }
	}

	@media (max-width: 1024px) {
		.stats-row {
			grid-template-columns: repeat(2, 1fr);
		}
	}

	@media (max-width: 640px) {
		.stats-row {
			grid-template-columns: 1fr;
		}
	}
</style>