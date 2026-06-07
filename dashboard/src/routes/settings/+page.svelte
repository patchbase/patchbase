<script lang="ts">
	import { onMount } from 'svelte';
	import AppLayout from '$lib/components/AppLayout.svelte';
	import { getSettings, updateSettings } from '$lib/api/settings.js';

	let globalPublicKey = $state('');
	let defaultSshUser = $state('');
	let loading = $state(true);
	let error = $state('');
	let activeTab = $state<'general' | 'integrations' | 'security'>('general');
	let copied = $state(false);
	let saving = $state(false);
	let saved = $state(false);

	async function loadSettings(): Promise<void> {
		loading = true;
		error = '';
		try {
			const data = await getSettings();
			globalPublicKey = data.global_ssh_public_key;
			defaultSshUser = data.default_ssh_pull_user || 'root';
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
			await updateSettings({ default_ssh_pull_user: defaultSshUser });
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

<style>
	.tabs-container {
		display: flex;
		border-bottom: 1px solid var(--border);
		gap: 16px;
		margin-bottom: 24px;
	}
	.tab-btn {
		background: none;
		border: none;
		color: var(--text-secondary);
		padding: 12px 4px;
		font-size: 15px;
		font-weight: 500;
		cursor: pointer;
		position: relative;
		transition: color 0.15s ease;
	}
	.tab-btn:hover {
		color: var(--text-primary);
	}
	.tab-btn.active {
		color: var(--accent);
	}
	.tab-btn.active::after {
		content: '';
		position: absolute;
		bottom: -1px;
		left: 0;
		right: 0;
		height: 2px;
		background: var(--accent);
	}

	.settings-card {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 24px;
		max-width: 800px;
		animation: fadeIn 0.2s ease-in-out;
	}

	.settings-title {
		font-size: 18px;
		font-weight: 600;
		margin-bottom: 8px;
		color: var(--text-primary);
	}

	.settings-description {
		font-size: 14px;
		color: var(--text-secondary);
		margin-bottom: 20px;
		line-height: 1.5;
	}

	.key-display {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.key-label {
		font-size: 13px;
		font-weight: 500;
		color: var(--text-secondary);
	}

	.key-textarea-wrapper {
		position: relative;
	}

	.key-textarea {
		width: 100%;
		height: 140px;
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 12px;
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-primary);
		resize: none;
		outline: none;
		line-height: 1.5;
		transition: border-color 0.15s ease;
	}

	.key-textarea:focus {
		border-color: var(--accent);
	}

	.key-actions {
		display: flex;
		gap: 12px;
		margin-top: 12px;
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
</style>

<AppLayout page="settings" title="System Settings">
	<div class="tabs-container">
		<button
			type="button"
			class="tab-btn"
			class:active={activeTab === 'general'}
			onclick={() => activeTab = 'general'}
		>
			General
		</button>
		<button
			type="button"
			class="tab-btn"
			class:active={activeTab === 'integrations'}
			onclick={() => activeTab = 'integrations'}
		>
			Integrations
		</button>
		<button
			type="button"
			class="tab-btn"
			class:active={activeTab === 'security'}
			onclick={() => activeTab = 'security'}
		>
			Security
		</button>
	</div>

	{#if loading}
		<div class="empty-state">
			<p>Loading settings...</p>
		</div>
	{:else if error}
		<div class="empty-state">
			<p style="color: var(--red); margin-bottom: 12px;">{error}</p>
			<button type="button" class="btn btn-secondary btn-sm" onclick={loadSettings}>
				Retry
			</button>
		</div>
	{:else}
		{#if activeTab === 'general'}
			<div class="settings-card" style="margin-bottom: 24px;">
				<h2 class="settings-title">Default SSH Pull User</h2>
				<p class="settings-description">
					This user will be pre-filled as the default when registering new SSH pull hosts.
				</p>
				<form onsubmit={(e) => { e.preventDefault(); void saveSettings(); }}>
					<div style="display: flex; gap: 12px; align-items: center;">
						<input
							type="text"
							class="form-input"
							style="max-width: 300px;"
							bind:value={defaultSshUser}
							placeholder="root"
							required
						/>
						<button type="submit" class="btn btn-primary" disabled={saving}>
							{saving ? 'Saving...' : 'Save'}
						</button>
						{#if saved}
							<span style="color: var(--green); font-size: 14px;">Saved!</span>
						{/if}
					</div>
				</form>
			</div>

			<div class="settings-card">
				<h2 class="settings-title">Global SSH Key</h2>
				<p class="settings-description">
					This key pair is used globally by default for SSH pull hosts. Add this public key to the <code>~/.ssh/authorized_keys</code> file on your target hosts to authorize PatchBase.
				</p>

				<div class="key-display">
					<span class="key-label">Global Public Key</span>
					<div class="key-textarea-wrapper">
						<textarea
							class="key-textarea"
							readonly
							value={globalPublicKey}
						></textarea>
					</div>
					<div class="key-actions">
						<button type="button" class="btn btn-primary" onclick={copyKey}>
							{copied ? 'Copied!' : 'Copy Public Key'}
						</button>
					</div>
				</div>
			</div>
		{:else if activeTab === 'integrations'}
			<div class="settings-card">
				<h2 class="settings-title">Third-party Integrations</h2>
				<p class="settings-description">
					Configure external integrations, webhook notifications, and alert systems here.
				</p>
				<div>
					<p style="color: var(--text-dim); font-style: italic;">No integrations configured yet.</p>
				</div>
			</div>
		{:else if activeTab === 'security'}
			<div class="settings-card">
				<h2 class="settings-title">Security Settings</h2>
				<p class="settings-description">
					Manage security levels, access controls, token expiration policies, and logging preferences.
				</p>
				<div>
					<p style="color: var(--text-dim); font-style: italic;">Access control settings are handled via Identity provider.</p>
				</div>
			</div>
		{/if}
	{/if}
</AppLayout>
