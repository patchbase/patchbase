<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
	interface Props {
		status: string;
	}

	let { status }: Props = $props();

	const colors: Record<string, string> = {
		reboot_host: 'red',
		restart_service: 'orange',
		update_package: 'blue',
		investigate: 'yellow',
		none: 'green',
		active: 'green',
		inactive: 'red',
		stale: 'yellow',
		critical: 'red',
		important: 'orange',
		moderate: 'yellow',
		low: 'green',
		affected_fix_available: 'blue',
		affected_no_fix: 'red',
		fixed_package_installed_pending_activation: 'orange',
		resolved: 'green',
		unknown: 'purple',
		security: 'red',
		bugfix: 'yellow',
		enhancement: 'blue',
		synced: 'green',
		not_configured: 'purple',
		success: 'green',
		failed: 'red',
		running: 'blue',
		pending: 'purple',
		waiting_approval: 'purple',
		rejected: 'red',
	};

	let color = $derived(colors[status] || 'purple');

	const labels: Record<string, string> = {
		reboot_host: 'Reboot required',
		restart_service: 'Restart required',
		update_package: 'Update available',
		investigate: 'Investigate',
		none: 'Clear',
		affected_fix_available: 'Fix available',
		affected_no_fix: 'No fix',
		fixed_package_installed_pending_activation: 'Pending activation',
		resolved: 'Resolved',
		success: 'Success',
		failed: 'Failed',
		running: 'Running',
		pending: 'Pending',
		waiting_approval: 'Needs approval',
		rejected: 'Rejected',
	};

	let display = $derived(labels[status] || status.replace(/_/g, ' '));
</script>

<span class="badge badge-{color}">
	<span class="badge-dot"></span>
	{display}
</span>