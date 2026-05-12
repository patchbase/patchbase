<script lang="ts">
	import type { Snippet } from 'svelte';
	import Logo from '$lib/components/Logo.svelte';

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
	];
</script>

<aside class="sidebar">
	<div class="sidebar-brand">
		<Logo size={28} />
		<span>patchbase</span>
		<span class="version">v0</span>
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