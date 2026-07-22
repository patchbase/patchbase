<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
	import Modal from '$lib/components/Modal.svelte';
	import { updateHost } from '$lib/api/hosts.js';
	import type { Host } from '$lib/types';

	interface Props {
		host: Host;
		class?: string;
		buttonText?: string;
		onUpdate?: (host: Host) => void;
		onError?: (err: Error) => void;
	}

	let { host, class: className = '', buttonText = 'Edit', onUpdate, onError }: Props = $props();
	let open = $state(false);
	let saving = $state(false);
	let error = $state('');
	let displayName = $state('');
	let pullHostname = $state('');
	let pullSSHUser = $state('');
	let pullFrequencyMinutes = $state(5);
	const formId = $derived(`edit-host-${host.id}`);

	function openModal(): void {
		displayName = host.display_name || '';
		pullHostname = host.configuration?.pull_hostname || '';
		pullSSHUser = host.configuration?.pull_ssh_user || '';
		pullFrequencyMinutes = host.configuration?.pull_frequency_minutes || 5;
		error = '';
		open = true;
	}

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		error = '';
		saving = true;
		try {
			const payload = { display_name: displayName.trim() };
			const updated = await updateHost(host.id, host.onboarding_mode === 'ssh' ? {
				...payload,
				pull_hostname: pullHostname.trim(),
				pull_ssh_user: pullSSHUser.trim(),
				pull_frequency_minutes: pullFrequencyMinutes
			} : payload);
			open = false;
			onUpdate?.(updated);
		} catch (err) {
			const updateError = err instanceof Error ? err : new Error(String(err));
			error = updateError.message;
			onError?.(updateError);
		} finally {
			saving = false;
		}
	}
</script>

<button type="button" class={className} onclick={openModal}>{buttonText}</button>

<Modal {open} title="Edit Host" onclose={() => { if (!saving) open = false; }} dismissible={!saving}>
	<form id={formId} onsubmit={submit}>
		{#if error}<div class="auth-error" style="margin-bottom:16px">{error}</div>{/if}
		<div class="form-group">
			<label for={`${formId}-display-name`}>Display name</label>
			<input id={`${formId}-display-name`} class="form-input" bind:value={displayName} required />
		</div>
		{#if host.onboarding_mode === 'ssh'}
			<div class="form-group">
				<label for={`${formId}-hostname`}>SSH hostname</label>
				<input id={`${formId}-hostname`} class="form-input" bind:value={pullHostname} required />
			</div>
			<div class="form-group">
				<label for={`${formId}-user`}>SSH user</label>
				<input id={`${formId}-user`} class="form-input" bind:value={pullSSHUser} required />
			</div>
			<div class="form-group">
				<label for={`${formId}-frequency`}>Pull frequency (minutes)</label>
				<input id={`${formId}-frequency`} class="form-input" type="number" min="5" bind:value={pullFrequencyMinutes} required />
			</div>
		{/if}
	</form>
	{#snippet footer()}
		<button type="button" class="btn btn-secondary btn-sm" onclick={() => { open = false; }} disabled={saving}>Cancel</button>
		<button type="submit" form={formId} class="btn btn-primary btn-sm" disabled={saving}>{saving ? 'Saving...' : 'Save'}</button>
	{/snippet}
</Modal>
