<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
	import { onMount } from 'svelte';
	import AppLayout from '$lib/components/AppLayout.svelte';
	import { getSettings, updateSettings, testEmail, sendReportNow } from '$lib/api/settings.js';
	import type { SMTPSettings } from '$lib/api/settings.js';

	let globalPublicKey = $state('');
	let defaultSshUser = $state('');
	let askToCopyPublicKey = $state(true);
	let loading = $state(true);
	let error = $state('');
	let activeTab = $state<'general' | 'integrations'>('general');
	let copied = $state(false);
	let saving = $state(false);
	let saved = $state(false);

	let smtpSettings = $state<SMTPSettings>({
		host: '',
		port: 587,
		username: '',
		password: '',
		from: '',
		report_hour: 9,
	});
	let emailFrequency = $state('never');
	let testEmailTo = $state('');
	let testEmailLoading = $state(false);
	let sendReportLoading = $state(false);
	let testEmailSuccess = $state(false);
	let reportSuccess = $state(false);

	async function loadSettings(): Promise<void> {
		loading = true;
		error = '';
		try {
			const data = await getSettings();
			globalPublicKey = data.global_ssh_public_key;
			defaultSshUser = data.default_ssh_pull_user || 'root';
			askToCopyPublicKey = data.ask_to_copy_public_key;
			if (data.smtp_settings) {
				smtpSettings = { ...data.smtp_settings, password: '' };
			}
			if (data.email_frequency) {
				emailFrequency = data.email_frequency;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load settings';
		} finally {
			loading = false;
		}
	}

	async function saveSettings(): Promise<void> {
		saving = true;
		error = '';
		saved = false;
		try {
			await updateSettings({
				default_ssh_pull_user: defaultSshUser,
				ask_to_copy_public_key: askToCopyPublicKey,
				smtp_settings: smtpSettings,
				email_frequency: emailFrequency,
			});
			saved = true;
			setTimeout(() => {
				saved = false;
			}, 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save settings';
		} finally {
			saving = false;
		}
	}

	async function handleTestEmail(): Promise<void> {
		if (!testEmailTo) {
			error = 'Test email address is required';
			return;
		}
		testEmailLoading = true;
		error = '';
		testEmailSuccess = false;
		try {
			await testEmail(testEmailTo);
			testEmailSuccess = true;
			setTimeout(() => (testEmailSuccess = false), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to send test email';
		} finally {
			testEmailLoading = false;
		}
	}

	async function handleSendReportNow(): Promise<void> {
		sendReportLoading = true;
		error = '';
		reportSuccess = false;
		try {
			await sendReportNow();
			reportSuccess = true;
			setTimeout(() => (reportSuccess = false), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to send report';
		} finally {
			sendReportLoading = false;
		}
	}

	onMount(() => {
		void loadSettings();
	});

	function copyKey(): void {
		if (!globalPublicKey) return;
		void navigator.clipboard.writeText(globalPublicKey);
		copied = true;
		setTimeout(() => {
			copied = false;
		}, 2000);
	}
</script>

<AppLayout page="settings" title="System Settings">
	<div class="settings-container">
		<!-- Modern Navigation Tabs -->
		<div class="tabs-header">
			<button
				type="button"
				class="tab-item"
				class:active={activeTab === 'general'}
				onclick={() => (activeTab = 'general')}
			>
				<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
					<circle cx="12" cy="12" r="3"></circle>
					<path
						d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
					></path>
				</svg>
				<span>General</span>
			</button>
			<button
				type="button"
				class="tab-item"
				class:active={activeTab === 'integrations'}
				onclick={() => (activeTab = 'integrations')}
			>
				<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
					<path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"></path>
					<polyline points="22,6 12,13 2,6"></polyline>
				</svg>
				<span>Integrations</span>
			</button>
		</div>

		{#if loading}
			<div class="empty-state">
				<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="spinner">
					<circle cx="12" cy="12" r="10" stroke-dasharray="32" stroke-dashoffset="10"></circle>
				</svg>
				<p>Loading settings...</p>
			</div>
		{:else}
			{#if error}
				<div class="alert alert-error">
					<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<circle cx="12" cy="12" r="10"></circle>
						<line x1="12" y1="8" x2="12" y2="12"></line>
						<line x1="12" y1="16" x2="12.01" y2="16"></line>
					</svg>
					<span>{error}</span>
					<button type="button" class="alert-close" onclick={() => (error = '')}>×</button>
				</div>
			{/if}

			{#if activeTab === 'general'}
				<div class="tab-content">
					<!-- Card 1: SSH Pull User Settings -->
					<div class="settings-card">
						<div class="card-header">
							<div class="card-icon">
								<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
									<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
									<circle cx="12" cy="7" r="4"></circle>
								</svg>
							</div>
							<div>
								<h3 class="card-title">Default SSH Pull User</h3>
								<p class="card-subtitle">
									Configure default parameters for SSH pull host registration
								</p>
							</div>
						</div>

						<form
							onsubmit={(e) => {
								e.preventDefault();
								void saveSettings();
							}}
						>
							<div class="form-group">
								<label for="ssh-user-input">Default SSH User</label>
								<div class="input-with-icon">
									<svg class="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
										<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
										<circle cx="12" cy="7" r="4"></circle>
									</svg>
									<input
										id="ssh-user-input"
										type="text"
										class="form-input"
										bind:value={defaultSshUser}
										placeholder="e.g. root or patchbase"
										required
									/>
								</div>
								<div class="form-hint">
									This user will be pre-filled automatically when registering new SSH pull hosts.
								</div>
							</div>

							<div class="setting-toggle-row">
								<label class="toggle-switch">
									<input type="checkbox" bind:checked={askToCopyPublicKey} />
									<span class="toggle-slider"></span>
								</label>
								<div class="toggle-info">
									<span class="toggle-title">Prompt to copy public key</span>
									<span class="toggle-desc">Automatically display a prompt to copy the public key after adding a new host</span>
								</div>
							</div>

							<div class="card-actions">
								<button type="submit" class="btn btn-primary" disabled={saving}>
									{#if saving}
										<svg class="spinner-sm" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
											<circle cx="12" cy="12" r="10" stroke-dasharray="32" stroke-dashoffset="10"></circle>
										</svg>
										Saving...
									{:else}
										Save General Settings
									{/if}
								</button>
								{#if saved}
									<span class="saved-badge">
										<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
											<polyline points="20 6 9 17 4 12"></polyline>
										</svg>
										Settings saved!
									</span>
								{/if}
							</div>
						</form>
					</div>

					<!-- Card 2: Global SSH Key -->
					<div class="settings-card">
						<div class="card-header">
							<div class="card-icon">
								<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
									<rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
									<path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
								</svg>
							</div>
							<div>
								<h3 class="card-title">Global Public SSH Key</h3>
								<p class="card-subtitle">
									Add this key to target hosts to enable secure automated patch monitoring
								</p>
							</div>
						</div>

						<div class="key-display">
							<div class="key-textarea-wrapper">
								<textarea
									class="key-textarea"
									readonly
									value={globalPublicKey}
									placeholder="No SSH key loaded"
								></textarea>
							</div>
							<div class="key-actions">
								<button type="button" class="btn btn-primary" onclick={copyKey}>
									{#if copied}
										<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width: 16px; height: 16px;">
											<polyline points="20 6 9 17 4 12"></polyline>
										</svg>
										Public Key Copied!
									{:else}
										<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width: 16px; height: 16px;">
											<rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
											<path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
										</svg>
										Copy Public Key
									{/if}
								</button>
							</div>
						</div>

						<div class="key-instruction-box">
							<span class="instruction-title">Quick Installation:</span>
							<code class="instruction-code">echo "{globalPublicKey ? globalPublicKey.slice(0, 40) + '...' : 'YOUR_PUBLIC_KEY'}" &gt;&gt; ~/.ssh/authorized_keys</code>
						</div>
					</div>
				</div>
			{:else if activeTab === 'integrations'}
				<div class="tab-content">
					<!-- Email / SMTP Settings Card -->
					<div class="settings-card">
						<div class="card-header">
							<div class="card-icon">
								<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
									<path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"></path>
									<polyline points="22,6 12,13 2,6"></polyline>
								</svg>
							</div>
							<div>
								<h3 class="card-title">Email Notifications (SMTP)</h3>
								<p class="card-subtitle">
									Configure server settings to receive scheduled patch reports and alerts
								</p>
							</div>
						</div>

						<form
							onsubmit={(e) => {
								e.preventDefault();
								void saveSettings();
							}}
						>
							<div class="form-grid-2">
								<div class="form-group">
									<label for="smtp-host">SMTP Host</label>
									<input
										id="smtp-host"
										type="text"
										class="form-input"
										bind:value={smtpSettings.host}
										placeholder="smtp.example.com"
									/>
								</div>
								<div class="form-group">
									<label for="smtp-port">Port</label>
									<input
										id="smtp-port"
										type="number"
										class="form-input"
										bind:value={smtpSettings.port}
										placeholder="587"
									/>
								</div>
								<div class="form-group">
									<label for="smtp-username">Username</label>
									<input
										id="smtp-username"
										type="text"
										class="form-input"
										bind:value={smtpSettings.username}
										placeholder="smtp_user"
									/>
								</div>
								<div class="form-group">
									<label for="smtp-password">Password</label>
									<input
										id="smtp-password"
										type="password"
										class="form-input"
										bind:value={smtpSettings.password}
										placeholder="••••••••••••"
									/>
									<div class="form-hint">Leave blank to keep unchanged</div>
								</div>
								<div class="form-group full-width">
									<label for="smtp-from">From Email Address</label>
									<input
										id="smtp-from"
										type="email"
										class="form-input"
										bind:value={smtpSettings.from}
										placeholder="noreply@example.com"
									/>
								</div>
								<div class="form-group">
									<label for="email-frequency">Report Frequency</label>
									<select id="email-frequency" class="form-input" bind:value={emailFrequency}>
										<option value="never">Never (Disabled)</option>
										<option value="daily">Daily Report</option>
									</select>
								</div>
								{#if emailFrequency === 'daily'}
									<div class="form-group">
										<label for="report-hour">Report Delivery Hour (UTC)</label>
										<select id="report-hour" class="form-input" bind:value={smtpSettings.report_hour}>
											{#each Array.from({ length: 24 }, (_, i) => i) as hour}
												<option value={hour}>{hour.toString().padStart(2, '0')}:00 UTC</option>
											{/each}
										</select>
									</div>
								{/if}
							</div>

							<div class="card-actions">
								<button type="submit" class="btn btn-primary" disabled={saving}>
									{#if saving}
										<svg class="spinner-sm" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
											<circle cx="12" cy="12" r="10" stroke-dasharray="32" stroke-dashoffset="10"></circle>
										</svg>
										Saving...
									{:else}
										Save SMTP Configuration
									{/if}
								</button>
								{#if saved}
									<span class="saved-badge">
										<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
											<polyline points="20 6 9 17 4 12"></polyline>
										</svg>
										Settings saved!
									</span>
								{/if}
							</div>
						</form>
					</div>

					<!-- Test & Manual Dispatch Card -->
					<div class="settings-card">
						<div class="card-header">
							<div class="card-icon">
								<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
									<polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"></polygon>
								</svg>
							</div>
							<div>
								<h3 class="card-title">Test & Manual Actions</h3>
								<p class="card-subtitle">
									Test your email configuration or trigger instant report generation
								</p>
							</div>
						</div>

						<div class="action-rows">
							<div class="action-item-card">
								<div class="action-item-info">
									<span class="action-item-title">Send Test Email</span>
									<span class="action-item-desc">Verify your SMTP server credentials by sending a test message</span>
								</div>
								<div class="action-item-controls">
									<input
										id="test-email"
										type="email"
										class="form-input"
										bind:value={testEmailTo}
										placeholder="admin@example.com"
									/>
									<button
										type="button"
										class="btn btn-secondary"
										onclick={handleTestEmail}
										disabled={testEmailLoading}
									>
										{#if testEmailLoading}
											<svg class="spinner-sm" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
												<circle cx="12" cy="12" r="10" stroke-dasharray="32" stroke-dashoffset="10"></circle>
											</svg>
											Sending...
										{:else}
											Send Test Email
										{/if}
									</button>
									{#if testEmailSuccess}
										<span class="saved-badge">
											<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
												<polyline points="20 6 9 17 4 12"></polyline>
											</svg>
											Sent!
										</span>
									{/if}
								</div>
							</div>

							<div class="action-item-card">
								<div class="action-item-info">
									<span class="action-item-title">Dispatch Report Now</span>
									<span class="action-item-desc">Immediately generate and deliver the latest patch status report</span>
								</div>
								<div class="action-item-controls">
									<button
										type="button"
										class="btn btn-secondary"
										onclick={handleSendReportNow}
										disabled={sendReportLoading}
									>
										{#if sendReportLoading}
											<svg class="spinner-sm" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
												<circle cx="12" cy="12" r="10" stroke-dasharray="32" stroke-dashoffset="10"></circle>
											</svg>
											Generating...
										{:else}
											Send Report Now
										{/if}
									</button>
									{#if reportSuccess}
										<span class="saved-badge">
											<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
												<polyline points="20 6 9 17 4 12"></polyline>
											</svg>
											Report sent!
										</span>
									{/if}
								</div>
							</div>
						</div>
					</div>
				</div>
			{/if}
		{/if}
	</div>
</AppLayout>

<style>
	.settings-container {
		display: flex;
		flex-direction: column;
		gap: 24px;
		width: 100%;
	}

	/* Tabs Header */
	.tabs-header {
		display: flex;
		align-items: center;
		gap: 8px;
		border-bottom: 1px solid var(--border);
		padding-bottom: 2px;
	}

	.tab-item {
		background: transparent;
		border: none;
		border-bottom: 2px solid transparent;
		color: var(--text-secondary);
		padding: 10px 18px;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
		display: flex;
		align-items: center;
		gap: 8px;
		transition: all var(--transition-fast);
		border-radius: var(--radius-sm) var(--radius-sm) 0 0;

		svg {
			width: 18px;
			height: 18px;
			color: var(--text-dim);
			transition: color var(--transition-fast);
		}

		&:hover {
			color: var(--text-primary);
			background: rgba(255, 255, 255, 0.03);

			svg {
				color: var(--text-primary);
			}
		}

		&.active {
			color: var(--accent-light);
			border-bottom-color: var(--accent);
			background: var(--accent-glow);

			svg {
				color: var(--accent-light);
			}
		}
	}

	/* Tab Content */
	.tab-content {
		display: flex;
		flex-direction: column;
		gap: 24px;
		animation: fadeIn 0.2s ease;
	}

	.settings-card {
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

	/* Forms & Inputs */
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
	}

	.form-grid-2 {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 20px;
		margin-bottom: 16px;
	}

	.full-width {
		grid-column: span 2;
	}

	/* Toggle Switch */
	.setting-toggle-row {
		display: flex;
		align-items: flex-start;
		gap: 14px;
		margin: 20px 0;
		padding: 14px 16px;
		background: rgba(0, 0, 0, 0.2);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
	}

	.toggle-switch {
		position: relative;
		display: inline-block;
		width: 44px;
		height: 24px;
		flex-shrink: 0;
		margin-top: 2px;

		input {
			opacity: 0;
			width: 0;
			height: 0;
		}
	}

	.toggle-slider {
		position: absolute;
		cursor: pointer;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background-color: rgba(255, 255, 255, 0.15);
		transition: 0.2s cubic-bezier(0.4, 0, 0.2, 1);
		border-radius: 24px;

		&::before {
			position: absolute;
			content: '';
			height: 18px;
			width: 18px;
			left: 3px;
			bottom: 3px;
			background-color: white;
			transition: 0.2s cubic-bezier(0.4, 0, 0.2, 1);
			border-radius: 50%;
		}
	}

	input:checked + .toggle-slider {
		background-color: var(--accent);
	}

	input:checked + .toggle-slider::before {
		transform: translateX(20px);
	}

	.toggle-info {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.toggle-title {
		font-size: 14px;
		font-weight: 600;
		color: var(--text-primary);
	}

	.toggle-desc {
		font-size: 12px;
		color: var(--text-dim);
	}

	.card-actions {
		display: flex;
		align-items: center;
		gap: 16px;
		margin-top: 24px;
		padding-top: 16px;
		border-top: 1px solid var(--border);
	}

	/* Public Key View */
	.key-display {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.key-textarea-wrapper {
		position: relative;
	}

	.key-textarea {
		width: 100%;
		height: 130px;
		background: #06070a;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 14px;
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-primary);
		resize: none;
		outline: none;
		line-height: 1.6;
		transition: border-color var(--transition-fast);

		&:focus {
			border-color: var(--accent);
		}
	}

	.key-actions {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.key-instruction-box {
		margin-top: 20px;
		padding: 12px 16px;
		background: rgba(0, 0, 0, 0.3);
		border: 1px dashed var(--border-light);
		border-radius: var(--radius-sm);
		display: flex;
		align-items: center;
		gap: 12px;
		flex-wrap: wrap;
	}

	.instruction-title {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-dim);
	}

	.instruction-code {
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--accent-light);
		word-break: break-all;
	}

	/* Integration Actions */
	.action-rows {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.action-item-card {
		background: rgba(0, 0, 0, 0.2);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 16px 20px;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 20px;
		flex-wrap: wrap;
	}

	.action-item-info {
		display: flex;
		flex-direction: column;
		gap: 4px;
		flex: 1;
		min-width: 240px;
	}

	.action-item-title {
		font-size: 14px;
		font-weight: 600;
		color: var(--text-primary);
	}

	.action-item-desc {
		font-size: 12px;
		color: var(--text-dim);
	}

	.action-item-controls {
		display: flex;
		align-items: center;
		gap: 12px;

		.form-input {
			width: 220px;
		}
	}

	/* Badges & Alerts */
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

	/* Spinners & Animations */
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

	@media (max-width: 640px) {
		.form-grid-2 {
			grid-template-columns: 1fr;
		}

		.full-width {
			grid-column: span 1;
		}

		.action-item-controls {
			width: 100%;
			flex-direction: column;
			align-items: stretch;

			.form-input {
				width: 100%;
			}
		}
	}
</style>


