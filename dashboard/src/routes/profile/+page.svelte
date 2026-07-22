<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
	import { onMount } from 'svelte';
	import AppLayout from '$lib/components/AppLayout.svelte';
	import { getProfile, updateProfile, type ProfileResponse } from '$lib/api/profile.js';
	import { getSession, setSession } from '$lib/auth/session.js';

	let loading = $state(true);
	let loadError = $state('');
	let formError = $state('');
	let email = $state('');
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let savingEmail = $state(false);
	let savingPassword = $state(false);
	let emailSaved = $state(false);
	let passwordSaved = $state(false);

	function updateSession(result: ProfileResponse): void {
		const existing = getSession();
		setSession({
			accessToken: result.access_token,
			passwordResetNeeded: existing?.passwordResetNeeded ?? false,
			user: {
				id: result.user.id,
				email: result.user.email,
				name: result.user.name,
			},
		});
	}

	async function loadProfile(): Promise<void> {
		loading = true;
		loadError = '';
		formError = '';
		try {
			const result = await getProfile();
			email = result.user.email;
			updateSession(result);
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'Failed to load profile.';
		} finally {
			loading = false;
		}
	}

	async function saveEmail(): Promise<void> {
		const nextEmail = email.trim();
		if (!nextEmail) {
			formError = 'Email is required.';
			return;
		}

		savingEmail = true;
		formError = '';
		emailSaved = false;
		try {
			const result = await updateProfile({ email: nextEmail });
			email = result.user.email;
			updateSession(result);
			emailSaved = true;
			setTimeout(() => {
				emailSaved = false;
			}, 3000);
		} catch (err) {
			formError = err instanceof Error ? err.message : 'Failed to update email.';
		} finally {
			savingEmail = false;
		}
	}

	async function savePassword(): Promise<void> {
		if (!currentPassword) {
			formError = 'Current password is required.';
			return;
		}
		if (newPassword.length < 12) {
			formError = 'New password must be at least 12 characters.';
			return;
		}
		if (newPassword !== confirmPassword) {
			formError = 'New password and confirmation do not match.';
			return;
		}

		savingPassword = true;
		formError = '';
		passwordSaved = false;
		try {
			const result = await updateProfile({
				current_password: currentPassword,
				new_password: newPassword,
			});
			updateSession(result);
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
			passwordSaved = true;
			setTimeout(() => {
				passwordSaved = false;
			}, 3000);
		} catch (err) {
			formError = err instanceof Error ? err.message : 'Failed to update password.';
		} finally {
			savingPassword = false;
		}
	}

	onMount(() => {
		void loadProfile();
	});
</script>

<style>
	.profile-grid {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		gap: 20px;
		max-width: 760px;
	}

	.profile-card {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 24px;
	}

	.profile-card h2 {
		font-size: 18px;
		font-weight: 600;
		margin-bottom: 16px;
	}

	.profile-actions {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-top: 20px;
	}

	.saved-message {
		color: var(--green);
		font-size: 14px;
	}

	.form-error {
		background: var(--red-bg);
		border: 1px solid rgba(248, 113, 113, 0.2);
		border-radius: 8px;
		color: var(--red);
		font-size: 13px;
		padding: 10px 12px;
	}
</style>

<AppLayout page="profile" title="Profile">
	{#if loading}
		<div class="empty-state">
			<p>Loading profile...</p>
		</div>
	{:else if loadError}
		<div class="empty-state">
			<p style="color: var(--red); margin-bottom: 12px;">{loadError}</p>
			<button type="button" class="btn btn-secondary btn-sm" onclick={loadProfile}>
				Retry
			</button>
		</div>
	{:else}
		<div class="profile-grid">
			{#if formError}
				<div class="form-error">{formError}</div>
			{/if}

			<form class="profile-card" onsubmit={(e) => { e.preventDefault(); void saveEmail(); }}>
				<h2>Email</h2>
				<div class="form-group">
					<label for="profile-email">Email</label>
					<input
						id="profile-email"
						type="email"
						class="form-input"
						bind:value={email}
						required
					/>
				</div>
				<div class="profile-actions">
					<button type="submit" class="btn btn-primary" disabled={savingEmail}>
						{savingEmail ? 'Saving...' : 'Save email'}
					</button>
					{#if emailSaved}
						<span class="saved-message">Saved</span>
					{/if}
				</div>
			</form>

			<form class="profile-card" onsubmit={(e) => { e.preventDefault(); void savePassword(); }}>
				<h2>Password</h2>
				<div class="form-group">
					<label for="profile-current-password">Current password</label>
					<input
						id="profile-current-password"
						type="password"
						class="form-input"
						bind:value={currentPassword}
						autocomplete="current-password"
						required
					/>
				</div>
				<div class="form-group">
					<label for="profile-new-password">New password</label>
					<input
						id="profile-new-password"
						type="password"
						class="form-input"
						bind:value={newPassword}
						autocomplete="new-password"
						required
					/>
					<div class="form-hint">Minimum 12 characters.</div>
				</div>
				<div class="form-group">
					<label for="profile-confirm-password">Confirm new password</label>
					<input
						id="profile-confirm-password"
						type="password"
						class="form-input"
						bind:value={confirmPassword}
						autocomplete="new-password"
						required
					/>
				</div>
				<div class="profile-actions">
					<button type="submit" class="btn btn-primary" disabled={savingPassword}>
						{savingPassword ? 'Saving...' : 'Save password'}
					</button>
					{#if passwordSaved}
						<span class="saved-message">Saved</span>
					{/if}
				</div>
			</form>
		</div>
	{/if}
</AppLayout>
