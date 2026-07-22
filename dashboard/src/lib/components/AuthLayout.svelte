<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
	import { getVersion } from '$lib/api/version.js';
	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		subtitle: string;
		children: Snippet;
	}

	let { title, subtitle, children }: Props = $props();
	let version = $state('...');

	$effect(() => {
		getVersion()
			.then((value) => (version = value))
			.catch(() => (version = 'unknown'));
	});
</script>

<div class="auth-page">
	<div class="auth-card">
		<div class="auth-brand">
			<a href="/" class="logo-link">
				<img src="/logo.png" alt="" class="logo-icon" width={26} height={26} />
				<span class="logo-text">PATCH<span class="logo-accent">BASE</span></span>
			</a>
			<span class="version">{version}</span>
		</div>
		<h1>{title}</h1>
		<p class="auth-subtitle">{subtitle}</p>
		{@render children()}
	</div>
</div>

<style>
	.auth-page {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 24px;
		background: radial-gradient(circle at top center, #131a29 0%, #090a0f 70%);
	}

	.auth-card {
		width: 100%;
		max-width: 460px;
		background: var(--bg-card);
		backdrop-filter: blur(16px);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: 0 28px 28px;
		box-shadow: var(--shadow-lg);
	}

	.auth-brand {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 20px 0 16px;
		margin-bottom: 18px;
		border-bottom: 1px solid var(--border);
	}

	.logo-link {
		display: flex;
		align-items: center;
		gap: 10px;
		text-decoration: none;
		color: var(--text-primary);
	}

	.logo-icon {
		border-radius: var(--radius-xs);
	}

	.logo-text {
		font-size: 18px;
		font-weight: 800;
		letter-spacing: 0.03em;
	}

	.logo-accent {
		color: var(--accent-light);
	}

	.version {
		font-size: 11px;
		color: var(--text-dim);
		font-family: var(--font-mono);
		margin-left: auto;
		background: rgba(255, 255, 255, 0.05);
		padding: 2px 6px;
		border-radius: var(--radius-xs);
		border: 1px solid var(--border);
	}

	.auth-card h1 {
		font-size: 24px;
		font-weight: 700;
		line-height: 1.2;
		letter-spacing: -0.02em;
		margin-bottom: 6px;
	}

	.auth-subtitle {
		color: var(--text-secondary);
		font-size: 14px;
		margin-bottom: 20px;
	}
</style>
