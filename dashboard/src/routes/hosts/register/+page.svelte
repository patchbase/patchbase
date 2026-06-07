<script lang="ts">
	import AppLayout from '$lib/components/AppLayout.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import { goto } from '$app/navigation';
	import {
		approveHost,
		createRegistrationToken,
		createSSHHost,
		listPendingHosts,
		listRegistrationTokens,
		revokeRegistrationToken,
		onboardSSHHost,
		createManualHost,
		ingestManualReport,
		type RegistrationTokenInfo,
	} from '$lib/api/hosts.js';
	import { getSettings } from '$lib/api/settings.js';
	import type { Host } from '$lib/types';
	import { getSession } from '$lib/auth/session.js';

	type Mode = 'agent' | 'ssh' | 'manual';
	type ManualCollectorOSFamily = 'apt' | 'rpm';

	let mode = $state<Mode>('agent');
	let loadingTokens = $state(true);
	let tokens = $state<RegistrationTokenInfo[]>([]);
	let tokenName = $state('');
	let newestToken = $state('');
	let error = $state('');
	let pendingHosts = $state<Host[]>([]);

	let sshDisplayName = $state('');
	let sshHostname = $state('');
	let sshUser = $state('root');
	let sshUserTouched = $state(false);
	let sshFrequencyMinutes = $state(360);
	let sshUniqueKeyPair = $state(false);
	let sshPublicKey = $state('');
	let sshHostID = $state('');
	let showSSHModal = $state(false);
	let sshCopied = $state(false);

	let manualDisplayName = $state('');
	let manualHostID = $state('');
	let manualOSFamily = $state<ManualCollectorOSFamily>('apt');
	let manualFileContent = $state('');
	let manualFileName = $state('');
	let manualStep = $state(1);
	let scriptContent = $state('');

	async function submitManualHost(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		error = '';
		try {
			const result = await createManualHost(manualDisplayName, '');
			manualHostID = result.host_id;
			manualStep = 2;
			void loadScriptContent();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create manual host.';
		}
	}

	function collectorScriptURL(): string {
		const params = new URLSearchParams({ os_family: manualOSFamily });
		return `/api/v1/hosts/manual/script?${params.toString()}`;
	}

	async function fetchScriptResponse(): Promise<Response> {
		const session = getSession();
		const response = await fetch(collectorScriptURL(), {
			headers: {
				Authorization: `Bearer ${session?.accessToken || ''}`
			}
		});
		if (!response.ok) {
			throw new Error(`Failed to fetch script: ${response.statusText}`);
		}
		return response;
	}

	async function loadScriptContent(): Promise<void> {
		try {
			const response = await fetchScriptResponse();
			scriptContent = await response.text();
		} catch (err) {
			console.error(err);
		}
	}

	async function downloadScript(): Promise<void> {
		error = '';
		try {
			const response = await fetchScriptResponse();
			const blob = await response.blob();
			const url = window.URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = 'patchbase-collector.sh';
			document.body.appendChild(a);
			a.click();
			a.remove();
			window.URL.revokeObjectURL(url);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to download script.';
		}
	}

	function handleFileChange(event: Event) {
		const input = event.target as HTMLInputElement;
		if (input.files && input.files[0]) {
			const file = input.files[0];
			manualFileName = file.name;
			const reader = new FileReader();
			reader.onload = (e) => {
				manualFileContent = e.target?.result as string || '';
			};
			reader.readAsText(file);
		}
	}

	async function submitReport(): Promise<void> {
		if (!manualFileContent) {
			error = 'Please select a report file first.';
			return;
		}
		error = '';
		try {
			await ingestManualReport(manualHostID, manualFileContent);
			void goto(`/hosts/${manualHostID}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to upload report.';
		}
	}

	async function refreshTokens(): Promise<void> {
		loadingTokens = true;
		error = '';
		try {
			tokens = await listRegistrationTokens();
			pendingHosts = await listPendingHosts();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load registration tokens.';
		} finally {
			loadingTokens = false;
		}
	}

	async function approvePendingHost(hostId: string): Promise<void> {
		error = '';
		try {
			await approveHost(hostId);
			await refreshTokens();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to approve host.';
		}
	}

	async function loadSettings(): Promise<void> {
		try {
			const settings = await getSettings();
			if (settings.default_ssh_pull_user && !sshUserTouched) {
				sshUser = settings.default_ssh_pull_user;
			}
		} catch (err) {
			console.error('Failed to load settings', err);
		}
	}

	$effect(() => {
		void refreshTokens();
		void loadSettings();
	});

	async function createToken(): Promise<void> {
		error = '';
		newestToken = '';
		try {
			const created = await createRegistrationToken(tokenName);
			newestToken = created.token;
			tokenName = '';
			await refreshTokens();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create registration token.';
		}
	}

	async function revokeToken(id: string): Promise<void> {
		error = '';
		try {
			await revokeRegistrationToken(id);
			await refreshTokens();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to revoke token.';
		}
	}

	async function submitSSHHost(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		error = '';
		sshPublicKey = '';
		sshHostID = '';
		sshCopied = false;
		try {
			const result = await createSSHHost({
				display_name: sshDisplayName,
				hostname: sshHostname,
				ssh_user: sshUser,
				frequency_minutes: sshFrequencyMinutes,
				unique_key_pair: sshUniqueKeyPair,
			});
			sshHostID = result.host_id;
			sshPublicKey = result.public_key;
			showSSHModal = true;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create SSH host.';
		}
	}

	function copySSHKey() {
		if (sshPublicKey) {
			navigator.clipboard.writeText(sshPublicKey).then(() => {
				sshCopied = true;
			}).catch((err) => {
				console.error('Failed to copy: ', err);
				sshCopied = true;
			});
		}
	}

	async function finishSSHRegistration(): Promise<void> {
		try {
			await onboardSSHHost(sshHostID);
			showSSHModal = false;
			void goto('/hosts');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to onboard SSH host.';
			showSSHModal = false;
		}
	}
</script>

<AppLayout page="hosts" title="Register Host">
	<div class="segmented-toggle" role="group" aria-label="Registration mode" style="margin-bottom:16px">
		<button type="button" class:active={mode === 'agent'} onclick={() => (mode = 'agent')}>Agent mode</button>
		<button type="button" class:active={mode === 'ssh'} onclick={() => (mode = 'ssh')}>SSH mode</button>
		<button type="button" class:active={mode === 'manual'} onclick={() => (mode = 'manual')}>Manual mode</button>
	</div>

	{#if error}
		<div class="auth-error" style="margin-bottom:16px">{error}</div>
	{/if}

	{#if mode === 'agent'}
		<div class="detail-card" style="margin-bottom:16px">
			<h3>Create Registration Token</h3>
			<p class="form-hint">Registration tokens are used only for enrollment. After registration, the agent stores only the issued host access token.</p>
			<div class="token-create-row">
				<input class="form-input" placeholder="Token name" bind:value={tokenName} />
				<button class="btn btn-primary btn-sm token-create-button" type="button" onclick={() => void createToken()}>Create token</button>
			</div>
			{#if newestToken}
				<div style="margin-top:12px">
					<p class="form-hint">Copy this token now (shown only once):</p>
					<pre class="mono" style="padding:10px;border:1px solid var(--border);border-radius:8px;overflow:auto">{newestToken}</pre>
					<p class="form-hint">Enrollment example: <span class="mono">patchbase-agent enroll https://your-server {newestToken}</span></p>
				</div>
			{/if}
		</div>

		<div class="detail-card">
			<h3>Registration Tokens</h3>
			{#if loadingTokens}
				<p>Loading tokens...</p>
			{:else if tokens.length === 0}
				<p>No tokens yet.</p>
			{:else}
				<table>
					<thead>
						<tr>
							<th>Name</th>
							<th>Created</th>
							<th>Last Used</th>
							<th>Status</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{#each tokens as token (token.id)}
							<tr>
								<td>{token.name}</td>
								<td>{token.created_at}</td>
								<td>{token.last_used_at || '-'}</td>
								<td>{token.revoked_at ? 'revoked' : 'active'}</td>
								<td>
									{#if !token.revoked_at}
										<button class="btn btn-secondary btn-sm" type="button" onclick={() => void revokeToken(token.id)}>Revoke</button>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>

		<div class="detail-card" style="margin-top:16px">
			<h3>Pending Approvals</h3>
			{#if pendingHosts.length === 0}
				<p>No hosts waiting approval.</p>
			{:else}
				<table>
					<thead>
						<tr>
							<th>Host</th>
							<th>Mode</th>
							<th>Created</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{#each pendingHosts as host (host.id)}
							<tr>
								<td>{host.display_name || host.hostname || host.id}</td>
								<td>{host.onboarding_mode || '-'}</td>
								<td>{host.created_at || '-'}</td>
								<td><button class="btn btn-primary btn-sm" type="button" onclick={() => void approvePendingHost(host.id)}>Approve</button></td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{:else if mode === 'ssh'}
		<form class="detail-card" onsubmit={submitSSHHost}>
			<h3>Create SSH Host</h3>
			<p class="form-hint">PatchBase will generate an SSH keypair. Add the public key on the host, then the system runs an immediate first collection attempt.</p>

			<div class="form-group">
				<label>Display Name</label>
				<input class="form-input" bind:value={sshDisplayName} placeholder="prod-db-01" />
			</div>
			<div class="form-group">
				<label>Hostname</label>
				<input class="form-input" bind:value={sshHostname} placeholder="db1.example.com" required />
			</div>
			<div class="form-group">
				<label>SSH User</label>
				<input class="form-input" bind:value={sshUser} oninput={() => { sshUserTouched = true; }} placeholder="root" required />
			</div>
			<div class="form-group">
				<label>Frequency Minutes</label>
				<input class="form-input" type="number" min="5" bind:value={sshFrequencyMinutes} />
			</div>
			<div class="form-group" style="display: flex; align-items: center; gap: 8px; margin-top: 12px; margin-bottom: 16px;">
				<input type="checkbox" id="sshUniqueKeyPair" bind:checked={sshUniqueKeyPair} style="width: 16px; height: 16px; accent-color: var(--accent); cursor: pointer;" />
				<label for="sshUniqueKeyPair" style="margin: 0; cursor: pointer; user-select: none;">Use a unique SSH key pair for this host</label>
			</div>

			<button class="btn btn-primary btn-sm" type="submit">Create SSH Host</button>
		</form>

		<Modal open={showSSHModal} title="SSH Public Key" dismissible={false}>
			<p class="form-hint" style="margin-bottom:8px;">Add this public key to <span class="mono">~/.ssh/authorized_keys</span> on the target host:</p>
			
			<div style="position: relative; margin-bottom: 16px;">
				<pre class="mono" style="padding:12px; padding-right:48px; border:1px solid var(--border); border-radius:8px; overflow:auto; white-space:pre-wrap; word-break:break-all; margin:0;">{sshPublicKey}</pre>
				<button 
					type="button" 
					class="copy-btn" 
					onclick={copySSHKey} 
					title="Copy to clipboard"
				>
					{#if sshCopied}
						<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
					{:else}
						<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
					{/if}
				</button>
			</div>

			{#snippet footer()}
				{#if sshCopied}
					<button class="btn btn-primary btn-sm" type="button" onclick={finishSSHRegistration}>
						Done
					</button>
				{/if}
			{/snippet}
		</Modal>
	{:else if mode === 'manual'}
		{#if manualStep === 1}
			<form class="detail-card" onsubmit={submitManualHost}>
				<h3>Create Manual Host</h3>
				<p class="form-hint">Create a manual host record. You can then run the collection script on the host and upload the results.</p>

				<div class="form-group">
					<label>Display Name</label>
					<input class="form-input" bind:value={manualDisplayName} placeholder="my-manual-server" required />
				</div>
				<div class="form-group">
					<label>Host OS Family</label>
					<select class="form-input" bind:value={manualOSFamily}>
						<option value="apt">Debian / Ubuntu (APT)</option>
						<option value="rpm">RHEL / Rocky / Alma / CentOS (RPM)</option>
					</select>
				</div>

				<button class="btn btn-primary btn-sm" type="submit">Create Manual Host</button>
			</form>
		{:else if manualStep === 2}
			<div class="detail-card">
				<h3>Onboard Manual Host</h3>
				<p class="form-hint">Follow the instructions below to capture package and system data from the target host and upload it here.</p>

				<div class="onboarding-steps" style="margin-top: 20px;">
					<div class="step-item" style="margin-bottom: 24px;">
						<h4>1. Download the Collector Script</h4>
						<p class="form-hint" style="margin-bottom: 12px;">This script gathers system metadata, package versions, and available updates without modifying your system.</p>
						<p class="form-hint" style="margin-bottom: 12px;">Selected OS family: <span class="mono">{manualOSFamily}</span></p>
						<button class="btn btn-secondary btn-sm" type="button" onclick={downloadScript}>
							Download Collector Script
						</button>
					</div>

					<div class="step-item" style="margin-bottom: 24px;">
						<h4>2. Run Script on Host</h4>
						<p class="form-hint" style="margin-bottom: 12px;">Upload the script to the target host and execute it, capturing all stdout to a file:</p>
						<pre class="mono" style="padding:12px; border:1px solid var(--border); border-radius:8px; overflow:auto; background:var(--bg-card); white-space:pre-wrap; word-break:break-all;">chmod +x patchbase-collector.sh
./patchbase-collector.sh > report.txt</pre>
				</div>

				<div class="step-item" style="margin-bottom: 24px;">
					<h4>3. Upload the Generated Report</h4>
					<p class="form-hint" style="margin-bottom: 12px;">Select the <span class="mono">report.txt</span> file generated from the step above.</p>
					<div style="display: flex; gap: 12px; align-items: center;">
						<input type="file" accept=".txt,.sh,.log" onchange={handleFileChange} class="form-input" style="max-width: 300px;" />
						<button class="btn btn-primary btn-sm" type="button" onclick={submitReport} disabled={!manualFileContent}>
							Upload Report
						</button>
					</div>
				</div>

				{#if scriptContent}
					<div class="step-item" style="margin-top: 32px;">
						<h4>Collector Script Contents (Preview)</h4>
						<pre class="mono" style="padding:12px; border:1px solid var(--border); border-radius:8px; max-height:250px; overflow:auto; background:var(--bg-card); font-size:12px;">{scriptContent}</pre>
					</div>
				{/if}
			</div>
		</div>
		{/if}
	{/if}
</AppLayout>

<style>
	.copy-btn {
		position: absolute;
		top: 8px;
		right: 8px;
		background: transparent;
		border: 1px solid transparent;
		border-radius: 4px;
		color: var(--text-muted);
		width: 32px;
		height: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		transition: all 0.2s;
	}
	.copy-btn:hover {
		color: var(--text-primary);
		background: var(--bg-card-hover);
		border-color: var(--border);
	}
</style>
