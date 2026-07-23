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
	let userName = $state('');
	let userId = $state('');
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let savingEmail = $state(false);
	let savingPassword = $state(false);
	let emailSaved = $state(false);
	let passwordSaved = $state(false);
	let showCurrentPassword = $state(false);
	let showNewPassword = $state(false);
	let showConfirmPassword = $state(false);
	let copiedId = $state(false);

	function updateSession(result: ProfileResponse): void {
		const existing = getSession();
		setSession({
			accessToken: result.access_token,
			passwordResetNeeded: existing?.passwordResetNeeded ?? false,
			user: {
				id: result.user.id,
				email: result.user.email,
				name: result.user.name,
				isAdmin: result.user.is_admin,
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
			userName = result.user.name || result.user.email.split('@')[0] || 'User';
			userId = result.user.id || '';
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
			userName = result.user.name || result.user.email.split('@')[0] || 'User';
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

	function copyUserId(): void {
		if (!userId) return;
		void navigator.clipboard.writeText(userId);
		copiedId = true;
		setTimeout(() => {
			copiedId = false;
		}, 2000);
	}

	let initials = $derived(
		(userName || email || 'U')
			.split(' ')
			.map((part) => part[0])
			.join('')
			.substring(0, 2)
			.toUpperCase(),
	);

	let isPasswordLengthValid = $derived(newPassword.length >= 12);
	let doPasswordsMatch = $derived(newPassword !== '' && newPassword === confirmPassword);

	onMount(() => {
		void loadProfile();
	});
</script>

<AppLayout page="profile" title="Account Profile">
	{#if loading}
		<div class="empty-state">
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="spinner">
				<circle cx="12" cy="12" r="10" stroke-dasharray="32" stroke-dashoffset="10"></circle>
			</svg>
			<p>Loading profile...</p>
		</div>
	{:else if loadError}
		<div class="empty-state">
			<svg viewBox="0 0 24 24" fill="none" stroke="var(--red)" stroke-width="1.5">
				<circle cx="12" cy="12" r="10"></circle>
				<line x1="12" y1="8" x2="12" y2="12"></line>
				<line x1="12" y1="16" x2="12.01" y2="16"></line>
			</svg>
			<p style="color: var(--red); margin-bottom: 16px;">{loadError}</p>
			<button type="button" class="btn btn-secondary btn-sm" onclick={loadProfile}>
				Retry
			</button>
		</div>
	{:else}
		<div class="profile-container">
			<!-- Hero User Banner -->
			<div class="user-hero-card">
				<div class="avatar-wrapper">
					<div class="avatar-circle">
						<span>{initials}</span>
					</div>
					<div class="status-indicator"></div>
				</div>
				<div class="user-hero-info">
					<div class="user-hero-title">
						<h2>{userName}</h2>
						<span class="badge badge-green">
							<span class="badge-dot"></span>
							Active Session
						</span>
					</div>
					<p class="user-hero-email">{email}</p>
					{#if userId}
						<div class="user-id-row">
							<span class="user-id-label">ID:</span>
							<code class="user-id-value">{userId}</code>
							<button type="button" class="copy-id-btn" onclick={copyUserId} title="Copy ID">
								{#if copiedId}
									<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
										<polyline points="20 6 9 17 4 12"></polyline>
									</svg>
								{:else}
									<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
										<rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
										<path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
									</svg>
								{/if}
							</button>
						</div>
					{/if}
				</div>
			</div>

			{#if formError}
				<div class="alert alert-error">
					<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<circle cx="12" cy="12" r="10"></circle>
						<line x1="12" y1="8" x2="12" y2="12"></line>
						<line x1="12" y1="16" x2="12.01" y2="16"></line>
					</svg>
					<span>{formError}</span>
					<button type="button" class="alert-close" onclick={() => (formError = '')}>×</button>
				</div>
			{/if}

			<div class="profile-grid">
				<!-- Left Column: Forms -->
				<div class="profile-main-col">
					<!-- Email Card -->
					<form class="profile-card" onsubmit={(e) => { e.preventDefault(); void saveEmail(); }}>
						<div class="card-header">
							<div class="card-icon">
								<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
									<path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"></path>
									<polyline points="22,6 12,13 2,6"></polyline>
								</svg>
							</div>
							<div>
								<h3 class="card-title">Email Address</h3>
								<p class="card-subtitle">Manage your account email used for notifications and sign in</p>
							</div>
						</div>

						<div class="form-group">
							<label for="profile-email">Email Address</label>
							<div class="input-with-icon">
								<svg class="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
									<path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"></path>
									<polyline points="22,6 12,13 2,6"></polyline>
								</svg>
								<input
									id="profile-email"
									type="email"
									class="form-input"
									bind:value={email}
									placeholder="name@example.com"
									required
								/>
							</div>
						</div>

						<div class="profile-actions">
							<button type="submit" class="btn btn-primary" disabled={savingEmail}>
								{#if savingEmail}
									<svg class="spinner-sm" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
										<circle cx="12" cy="12" r="10" stroke-dasharray="32" stroke-dashoffset="10"></circle>
									</svg>
									Saving...
								{:else}
									Save Email
								{/if}
							</button>
							{#if emailSaved}
								<span class="saved-badge">
									<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
										<polyline points="20 6 9 17 4 12"></polyline>
									</svg>
									Email updated successfully
								</span>
							{/if}
						</div>
					</form>

					<!-- Password Card -->
					<form class="profile-card" onsubmit={(e) => { e.preventDefault(); void savePassword(); }}>
						<div class="card-header">
							<div class="card-icon">
								<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
									<rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
									<path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
								</svg>
							</div>
							<div>
								<h3 class="card-title">Password & Security</h3>
								<p class="card-subtitle">Update your password to keep your account secure</p>
							</div>
						</div>

						<div class="form-group">
							<label for="profile-current-password">Current Password</label>
							<div class="input-with-icon">
								<svg class="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
									<rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
									<path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
								</svg>
								<input
									id="profile-current-password"
									type={showCurrentPassword ? 'text' : 'password'}
									class="form-input"
									bind:value={currentPassword}
									autocomplete="current-password"
									placeholder="Enter current password"
									required
								/>
								<button
									type="button"
									class="toggle-password-btn"
									onclick={() => (showCurrentPassword = !showCurrentPassword)}
									title={showCurrentPassword ? 'Hide password' : 'Show password'}
								>
									{#if showCurrentPassword}
										<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
											<path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path>
											<line x1="1" y1="1" x2="23" y2="23"></line>
										</svg>
									{:else}
										<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
											<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
											<circle cx="12" cy="12" r="3"></circle>
										</svg>
									{/if}
								</button>
							</div>
						</div>

						<div class="form-row">
							<div class="form-group">
								<label for="profile-new-password">New Password</label>
								<div class="input-with-icon">
									<svg class="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
										<path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"></path>
									</svg>
									<input
										id="profile-new-password"
										type={showNewPassword ? 'text' : 'password'}
										class="form-input"
										bind:value={newPassword}
										autocomplete="new-password"
										placeholder="Min. 12 characters"
										required
									/>
									<button
										type="button"
										class="toggle-password-btn"
										onclick={() => (showNewPassword = !showNewPassword)}
										title={showNewPassword ? 'Hide password' : 'Show password'}
									>
										{#if showNewPassword}
											<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
												<path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path>
												<line x1="1" y1="1" x2="23" y2="23"></line>
											</svg>
										{:else}
											<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
												<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
												<circle cx="12" cy="12" r="3"></circle>
											</svg>
										{/if}
									</button>
								</div>
							</div>

							<div class="form-group">
								<label for="profile-confirm-password">Confirm Password</label>
								<div class="input-with-icon">
									<svg class="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
										<polyline points="20 6 9 17 4 12"></polyline>
									</svg>
									<input
										id="profile-confirm-password"
										type={showConfirmPassword ? 'text' : 'password'}
										class="form-input"
										bind:value={confirmPassword}
										autocomplete="new-password"
										placeholder="Re-enter password"
										required
									/>
									<button
										type="button"
										class="toggle-password-btn"
										onclick={() => (showConfirmPassword = !showConfirmPassword)}
										title={showConfirmPassword ? 'Hide password' : 'Show password'}
									>
										{#if showConfirmPassword}
											<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
												<path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path>
												<line x1="1" y1="1" x2="23" y2="23"></line>
											</svg>
										{:else}
											<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
												<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
												<circle cx="12" cy="12" r="3"></circle>
											</svg>
										{/if}
									</button>
								</div>
							</div>
						</div>

						<!-- Password Policy Requirements Box -->
						{#if newPassword.length > 0}
							<div class="password-checklist">
								<div class="check-item" class:valid={isPasswordLengthValid}>
									<span class="check-icon">{isPasswordLengthValid ? '✓' : '•'}</span>
									<span>At least 12 characters</span>
								</div>
								<div class="check-item" class:valid={doPasswordsMatch}>
									<span class="check-icon">{doPasswordsMatch ? '✓' : '•'}</span>
									<span>Passwords match</span>
								</div>
							</div>
						{/if}

						<div class="profile-actions">
							<button type="submit" class="btn btn-primary" disabled={savingPassword}>
								{#if savingPassword}
									<svg class="spinner-sm" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
										<circle cx="12" cy="12" r="10" stroke-dasharray="32" stroke-dashoffset="10"></circle>
									</svg>
									Updating...
								{:else}
									Update Password
								{/if}
							</button>
							{#if passwordSaved}
								<span class="saved-badge">
									<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
										<polyline points="20 6 9 17 4 12"></polyline>
									</svg>
									Password changed successfully
								</span>
							{/if}
						</div>
					</form>
				</div>

				<!-- Right Column: Security & Summary Stats -->
				<div class="profile-side-col">
					<div class="side-card">
						<div class="side-card-title">
							<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
								<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
							</svg>
							<span>Security Overview</span>
						</div>
						<div class="side-card-body">
							<div class="side-info-row">
								<span class="label">Authentication</span>
								<span class="value">Internal User</span>
							</div>
							<div class="side-info-row">
								<span class="label">Token Status</span>
								<span class="badge badge-green">Valid</span>
							</div>
							<div class="side-info-row">
								<span class="label">Password Policy</span>
								<span class="value">12+ Chars</span>
							</div>
						</div>
					</div>

					<div class="side-card hint-card">
						<div class="side-card-title">
							<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
								<circle cx="12" cy="12" r="10"></circle>
								<line x1="12" y1="16" x2="12" y2="12"></line>
								<line x1="12" y1="8" x2="12.01" y2="8"></line>
							</svg>
							<span>Security Tip</span>
						</div>
						<p class="hint-text">
							Ensure your password uses a combination of upper and lowercase letters, numbers, and symbols for optimum security.
						</p>
					</div>
				</div>
			</div>
		</div>
	{/if}
</AppLayout>

<style>
	.profile-container {
		display: flex;
		flex-direction: column;
		gap: 24px;
		width: 100%;
	}

	/* Hero Card */
	.user-hero-card {
		background: linear-gradient(135deg, rgba(15, 18, 26, 0.85) 0%, rgba(26, 32, 46, 0.7) 100%);
		backdrop-filter: blur(16px);
		border: 1px solid var(--border-light);
		border-radius: var(--radius-lg);
		padding: 28px 32px;
		display: flex;
		align-items: center;
		gap: 24px;
		box-shadow: var(--shadow-md);
		position: relative;
		overflow: hidden;

		&::before {
			content: '';
			position: absolute;
			top: -40px;
			right: -40px;
			width: 180px;
			height: 180px;
			background: radial-gradient(circle, var(--accent-glow) 0%, transparent 70%);
			pointer-events: none;
			border-radius: 50%;
		}
	}

	.avatar-wrapper {
		position: relative;
		flex-shrink: 0;
	}

	.avatar-circle {
		width: 72px;
		height: 72px;
		border-radius: 50%;
		background: linear-gradient(135deg, var(--accent) 0%, var(--accent-dark) 100%);
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 26px;
		font-weight: 700;
		color: #030712;
		box-shadow: 0 0 20px rgba(34, 197, 94, 0.3);
		border: 2px solid rgba(255, 255, 255, 0.2);
	}

	.status-indicator {
		position: absolute;
		bottom: 2px;
		right: 2px;
		width: 14px;
		height: 14px;
		background: var(--accent);
		border: 2.5px solid #0f121a;
		border-radius: 50%;
	}

	.user-hero-info {
		display: flex;
		flex-direction: column;
		gap: 6px;
		flex: 1;
		min-width: 0;
	}

	.user-hero-title {
		display: flex;
		align-items: center;
		gap: 12px;
		flex-wrap: wrap;

		h2 {
			font-size: 22px;
			font-weight: 700;
			letter-spacing: -0.02em;
			color: var(--text-primary);
			margin: 0;
		}
	}

	.user-hero-email {
		font-size: 14px;
		color: var(--text-secondary);
		margin: 0;
	}

	.user-id-row {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		margin-top: 4px;
		font-size: 12px;
	}

	.user-id-label {
		color: var(--text-dim);
		font-weight: 600;
	}

	.user-id-value {
		font-family: var(--font-mono);
		color: var(--text-secondary);
		background: rgba(0, 0, 0, 0.3);
		padding: 2px 8px;
		border-radius: var(--radius-xs);
		border: 1px solid var(--border);
	}

	.copy-id-btn {
		background: transparent;
		border: none;
		color: var(--text-dim);
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 2px;
		border-radius: var(--radius-xs);
		transition: color var(--transition-fast);

		svg {
			width: 14px;
			height: 14px;
		}

		&:hover {
			color: var(--accent-light);
		}
	}

	/* Alerts */
	.alert {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 14px 18px;
		border-radius: var(--radius-md);
		font-size: 14px;
		line-height: 1.4;
		animation: fadeIn 0.2s ease;

		svg {
			width: 18px;
			height: 18px;
			flex-shrink: 0;
		}
	}

	.alert-error {
		background: var(--red-bg);
		border: 1px solid var(--red-border);
		color: var(--red);
	}

	.alert-close {
		margin-left: auto;
		background: none;
		border: none;
		color: currentColor;
		font-size: 18px;
		cursor: pointer;
		opacity: 0.7;

		&:hover {
			opacity: 1;
		}
	}

	/* Grid Layout */
	.profile-grid {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 300px;
		gap: 24px;
		align-items: start;
	}

	.profile-main-col {
		display: flex;
		flex-direction: column;
		gap: 24px;
	}

	.profile-card {
		background: var(--bg-card);
		backdrop-filter: blur(12px);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 24px;
		transition: border-color var(--transition-fast);

		&:hover {
			border-color: var(--border-light);
		}
	}

	.card-header {
		display: flex;
		align-items: flex-start;
		gap: 14px;
		margin-bottom: 24px;
		padding-bottom: 16px;
		border-bottom: 1px solid var(--border);
	}

	.card-icon {
		width: 40px;
		height: 40px;
		border-radius: var(--radius-sm);
		background: rgba(255, 255, 255, 0.04);
		border: 1px solid var(--border);
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--accent-light);
		flex-shrink: 0;

		svg {
			width: 20px;
			height: 20px;
		}
	}

	.card-title {
		font-size: 16px;
		font-weight: 600;
		color: var(--text-primary);
		margin-bottom: 4px;
	}

	.card-subtitle {
		font-size: 13px;
		color: var(--text-secondary);
		margin: 0;
	}

	/* Form Elements */
	.input-with-icon {
		position: relative;
		display: flex;
		align-items: center;
	}

	.input-icon {
		position: absolute;
		left: 12px;
		width: 16px;
		height: 16px;
		color: var(--text-dim);
		pointer-events: none;
	}

	.input-with-icon .form-input {
		padding-left: 38px;
		padding-right: 38px;
	}

	.toggle-password-btn {
		position: absolute;
		right: 10px;
		background: transparent;
		border: none;
		color: var(--text-dim);
		cursor: pointer;
		padding: 4px;
		display: flex;
		align-items: center;
		justify-content: center;
		transition: color var(--transition-fast);

		svg {
			width: 16px;
			height: 16px;
		}

		&:hover {
			color: var(--text-primary);
		}
	}

	.password-checklist {
		margin-top: 12px;
		padding: 12px;
		background: rgba(0, 0, 0, 0.25);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.check-item {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--text-dim);
		transition: color var(--transition-fast);

		&.valid {
			color: var(--green);

			.check-icon {
				font-weight: bold;
			}
		}
	}

	.check-icon {
		width: 14px;
		text-align: center;
	}

	.profile-actions {
		display: flex;
		align-items: center;
		gap: 16px;
		margin-top: 24px;
	}

	.saved-badge {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		color: var(--green);
		font-size: 13px;
		font-weight: 500;
		animation: fadeIn 0.2s ease;

		svg {
			width: 15px;
			height: 15px;
		}
	}

	/* Side Column */
	.profile-side-col {
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	.side-card {
		background: var(--bg-card);
		backdrop-filter: blur(12px);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 20px;
	}

	.side-card-title {
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: 14px;
		font-weight: 600;
		color: var(--text-primary);
		margin-bottom: 16px;

		svg {
			width: 18px;
			height: 18px;
			color: var(--accent-light);
		}
	}

	.side-card-body {
		display: flex;
		flex-direction: flex-direction;
		flex-direction: column;
		gap: 12px;
	}

	.side-info-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		font-size: 13px;
		padding-bottom: 8px;
		border-bottom: 1px solid var(--border);

		&:last-child {
			border-bottom: none;
			padding-bottom: 0;
		}

		.label {
			color: var(--text-dim);
		}

		.value {
			color: var(--text-secondary);
			font-family: var(--font-mono);
		}
	}

	.hint-card {
		background: rgba(34, 197, 94, 0.03);
		border-color: rgba(34, 197, 94, 0.15);
	}

	.hint-text {
		font-size: 13px;
		color: var(--text-secondary);
		line-height: 1.5;
		margin: 0;
	}

	/* Animations & Spinners */
	.spinner,
	.spinner-sm {
		animation: rotate 1s linear infinite;
	}

	.spinner {
		width: 32px;
		height: 32px;
	}

	.spinner-sm {
		width: 14px;
		height: 14px;
	}

	@keyframes rotate {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}

	@keyframes fadeIn {
		from {
			opacity: 0;
			transform: translateY(4px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	@media (max-width: 860px) {
		.profile-grid {
			grid-template-columns: 1fr;
		}

		.user-hero-card {
			flex-direction: column;
			text-align: center;
			padding: 24px 20px;
		}

		.user-hero-title {
			justify-content: center;
		}
	}
</style>

