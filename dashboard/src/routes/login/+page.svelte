<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
	import { goto } from '$app/navigation';
	import { getSetupStatus, login } from '$lib/api/auth.js';
	import { setSessionFromLogin } from '$lib/auth/session.js';
	import AuthLayout from '$lib/components/AuthLayout.svelte';
	import { onMount } from 'svelte';

	let setupCompleted = $state(true);
	let email = $state('');
	let password = $state('');
	let loading = $state(false);
	let error = $state('');

	onMount(() => {
		const run = async (): Promise<void> => {
			try {
				const status = await getSetupStatus();
				setupCompleted = status.completed;
			} catch {
				setupCompleted = true;
			}
		};

		void run();
	});

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		error = '';
		loading = true;

		try {
			const result = await login(email, password);
			setSessionFromLogin(result);

			if (!result.setup_completed || result.password_reset_needed) {
				await goto('/setup', { replaceState: true });
				return;
			}

			await goto('/', { replaceState: true });
		} catch (err) {
			error = err instanceof Error ? err.message : 'Sign in failed.';
		} finally {
			loading = false;
		}
	}
</script>

<AuthLayout title="Sign In" subtitle="Enter your admin credentials to access the dashboard.">
	{#if setupCompleted === false}
		<div class="auth-message">
			Initial setup is not complete yet. Use the bootstrap admin credentials from server logs.
		</div>
	{/if}

	{#if error}
		<div class="auth-error">{error}</div>
	{/if}

	<form class="auth-form" onsubmit={submit}>
		<div class="form-group">
			<label for="login-email">Email</label>
			<input
				id="login-email"
				type="email"
				class="form-input"
				bind:value={email}
				placeholder="admin@patchbase.local"
				required
			/>
		</div>

		<div class="form-group">
			<label for="login-password">Password</label>
			<input
				id="login-password"
				type="password"
				class="form-input"
				bind:value={password}
				placeholder="Enter your password"
				required
			/>
		</div>

		<button type="submit" class="btn btn-primary auth-submit" disabled={loading}>
			{loading ? 'Signing in...' : 'Sign in'}
		</button>
	</form>
</AuthLayout>
