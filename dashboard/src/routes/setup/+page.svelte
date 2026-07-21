// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
<script lang="ts">
	import { goto } from '$app/navigation';
	import { completeSetup } from '$lib/api/auth.js';
	import { clearSession, getSession, setSessionFromLogin } from '$lib/auth/session.js';
	import AuthLayout from '$lib/components/AuthLayout.svelte';
	import { onMount } from 'svelte';

	let name = $state('Administrator');
	let email = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let saving = $state(false);
	let error = $state('');

	onMount(() => {
		const session = getSession();
		if (!session) {
			void goto('/login', { replaceState: true });
			return;
		}

		name = session.user.name || 'Administrator';
		email = session.user.email;
	});

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		error = '';

		if (password.length < 12) {
			error = 'Password must be at least 12 characters.';
			return;
		}
		if (password !== confirmPassword) {
			error = 'Passwords do not match.';
			return;
		}

		const session = getSession();
		if (!session) {
			await goto('/login', { replaceState: true });
			return;
		}

		saving = true;
		try {
			const result = await completeSetup(session.accessToken, {
				name,
				email,
				password,
			});
			setSessionFromLogin(result);
			await goto('/', { replaceState: true });
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Setup failed.';
			error = message;
			if (message.toLowerCase().includes('unauthorized')) {
				clearSession();
				await goto('/login', { replaceState: true });
			}
		} finally {
			saving = false;
		}
	}
</script>

<AuthLayout
	title="Finish Setup"
	subtitle="Set the permanent admin name, email, and password before entering the dashboard."
>
	{#if error}
		<div class="auth-error">{error}</div>
	{/if}

	<form class="auth-form" onsubmit={submit}>
		<div class="form-group">
			<label for="setup-name">Name</label>
			<input id="setup-name" type="text" class="form-input" bind:value={name} required />
		</div>

		<div class="form-group">
			<label for="setup-email">Email</label>
			<input id="setup-email" type="email" class="form-input" bind:value={email} required />
		</div>

		<div class="form-group">
			<label for="setup-password">New password</label>
			<input
				id="setup-password"
				type="password"
				class="form-input"
				bind:value={password}
				placeholder="At least 12 characters"
				required
			/>
			<div class="form-hint">This replaces the temporary bootstrap password.</div>
		</div>

		<div class="form-group">
			<label for="setup-password-confirm">Confirm password</label>
			<input id="setup-password-confirm" type="password" class="form-input" bind:value={confirmPassword} required />
		</div>

		<button type="submit" class="btn btn-primary auth-submit" disabled={saving}>
			{saving ? 'Saving...' : 'Complete setup'}
		</button>
	</form>
</AuthLayout>
