<script lang="ts">
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
