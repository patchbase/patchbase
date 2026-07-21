// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
<script lang="ts">
	interface Props {
		size?: number;
	}

	let { size = 28 }: Props = $props();
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
	<path d="M12 22C12 22 4 16 4 10C4 5.58 7.58 2 12 2C16.42 2 20 5.58 20 10C20 16 12 22 12 22Z" stroke="#E8EAED" stroke-width="1.5" fill="none"/>
	<circle cx="12" cy="10" r="3" fill="#22C55E" opacity="0.9"/>
	<circle cx="12" cy="10" r="3" stroke="#4ADE80" stroke-width="1.5" fill="none"/>
</svg>