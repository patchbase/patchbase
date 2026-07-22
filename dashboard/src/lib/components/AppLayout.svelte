<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
	import type { Snippet } from 'svelte';
	import { getVersion } from '$lib/api/version.js';

	interface NavItem {
		page: string;
		label: string;
		href: string;
		icon: string;
	}

	interface Props {
		page: string;
		title: string;
		children: Snippet;
		actions?: Snippet;
	}

	let { page, title, children, actions }: Props = $props();
	let version = $state('...');

	$effect(() => {
		getVersion()
			.then((value) => (version = value))
			.catch(() => (version = 'unknown'));
	});

	const nav: { section: string; items: NavItem[] }[] = [
		{
			section: 'Overview',
			items: [
				{
					page: 'dashboard',
					label: 'Dashboard',
					href: '/',
					icon:
						'<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>',
				},
			],
		},
		{
			section: 'Fleet',
			items: [
				{
					page: 'hosts',
					label: 'Hosts',
					href: '/hosts',
					icon: '<rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><circle cx="6" cy="6" r="1"/><circle cx="6" cy="18" r="1"/>',
				},
				{
					page: 'advisories',
					label: 'Advisories',
					href: '/advisories',
					icon: '<path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>',
				},
				{
					page: 'reboots',
					label: 'Pending Reboots',
					href: '/reboots',
					icon: '<polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-8.36L23 10"/>',
				},
			],
		},
		{
			section: 'System',
			items: [
				{
					page: 'profile',
					label: 'Profile',
					href: '/profile',
					icon: '<circle cx="12" cy="8" r="4"/><path d="M20 21a8 8 0 0 0-16 0"/>',
				},
				{
					page: 'settings',
					label: 'Settings',
					href: '/settings',
					icon: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>',
				},
				{
					page: 'audit-log',
					label: 'Audit Log',
					href: '/audit-log',
					icon: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="9" y1="13" x2="15" y2="13"/><line x1="9" y1="17" x2="15" y2="17"/>',
				},
			],
		},
	];
</script>

<aside class="sidebar">
	<div class="sidebar-brand">
		<a href="/" class="logo-link">
			<img src="/logo.png" alt="" class="logo-icon" width={28} height={28} />
			<span class="logo-text">PATCH<span class="logo-accent">BASE</span></span>
		</a>
		<span class="version">{version}</span>
	</div>
	<nav class="sidebar-nav">
		{#each nav as group}
			<div class="nav-section-title">{group.section}</div>
			{#each group.items as item}
				<a href={item.href} class:active={page === item.page}>
					<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
						{@html item.icon}
					</svg>
					{item.label}
				</a>
			{/each}
		{/each}
	</nav>
	<div class="sidebar-footer">
		<a href="https://github.com/patchbase" target="_blank" rel="noopener">GitHub</a>
	</div>
</aside>

<div class="main">
	<div class="page-header">
		<h1>{title}</h1>
		{#if actions}
			{@render actions()}
		{/if}
	</div>
	<div class="page-content">
		{@render children()}
	</div>
</div>

<style>
	.sidebar {
		position: fixed;
		top: 0;
		left: 0;
		bottom: 0;
		width: var(--sidebar-width);
		background: var(--bg-sidebar);
		border-right: 1px solid var(--border);
		display: flex;
		flex-direction: column;
		z-index: 50;
		overflow-y: auto;
	}

	.sidebar-brand {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 20px 20px 16px;
		border-bottom: 1px solid var(--border);
	}

	.logo-link {
		display: flex;
		align-items: center;
		gap: 10px;
		text-decoration: none;
		color: var(--text-primary);
	}

	.logo-link:hover {
		text-decoration: none;
	}

	.logo-icon {
		border-radius: var(--radius-sm);
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

	.sidebar-nav {
		padding: 16px 0;
		flex: 1;
	}

	.nav-section-title {
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--text-dim);
		padding: 16px 20px 8px;
	}

	.sidebar-nav a {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 9px 20px;
		color: var(--text-secondary);
		font-size: 14px;
		font-weight: 500;
		transition: all var(--transition-fast);
		border-left: 3px solid transparent;
		text-decoration: none;
	}

	.sidebar-nav a:hover {
		color: var(--text-primary);
		background: rgba(255, 255, 255, 0.03);
		text-decoration: none;
	}

	.sidebar-nav a.active {
		color: var(--accent-light);
		background: var(--accent-glow);
		border-left-color: var(--accent);
		font-weight: 600;
	}

	.sidebar-nav a svg {
		width: 18px;
		height: 18px;
		flex-shrink: 0;
		opacity: 0.7;
		transition: opacity var(--transition-fast);
	}

	.sidebar-nav a:hover svg,
	.sidebar-nav a.active svg {
		opacity: 1;
	}

	.sidebar-footer {
		padding: 16px 20px;
		border-top: 1px solid var(--border);
		font-size: 12px;
		color: var(--text-dim);
	}

	.sidebar-footer a {
		color: var(--text-dim);
	}

	.sidebar-footer a:hover {
		color: var(--accent-light);
	}

	.main {
		margin-left: var(--sidebar-width);
		flex: 1;
		min-width: 0;
	}

	.page-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 24px 32px;
		border-bottom: 1px solid var(--border);
		background: var(--bg-secondary);
		backdrop-filter: blur(12px);
		position: sticky;
		top: 0;
		z-index: 40;
	}

	.page-header h1 {
		font-size: 22px;
		font-weight: 700;
		letter-spacing: -0.02em;
	}

	.page-content {
		padding: 28px 32px;
	}

	@media (max-width: 768px) {
		.sidebar {
			display: none;
		}

		.main {
			margin-left: 0;
		}

		.page-header {
			padding: 16px 20px;
		}

		.page-content {
			padding: 20px 16px;
		}
	}
</style>
