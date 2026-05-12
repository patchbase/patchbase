<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		subtitle: string;
		children: Snippet;
	}

	let { title, subtitle, children }: Props = $props();
</script>

<div class="auth-page">
	<div class="auth-card">
		<h1>{title}</h1>
		<p class="auth-subtitle">{subtitle}</p>
		{@render children()}
	</div>
</div>
