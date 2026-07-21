// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
<script lang="ts">
    import { onMount } from "svelte";
    import AppLayout from "$lib/components/AppLayout.svelte";
    import { getSettings, updateSettings, testEmail, sendReportNow } from "$lib/api/settings.js";
    import type { SMTPSettings } from "$lib/api/settings.js";

    let globalPublicKey = $state("");
    let defaultSshUser = $state("");
    let askToCopyPublicKey = $state(true);
    let loading = $state(true);
    let error = $state("");
    let activeTab = $state<"general" | "integrations" | "security">("general");
    let copied = $state(false);
    let saving = $state(false);
    let saved = $state(false);

    let smtpSettings = $state<SMTPSettings>({
        host: "",
        port: 587,
        username: "",
        password: "",
        from: "",
        report_hour: 9,
    });
    let emailFrequency = $state("never");
    let testEmailTo = $state("");
    let testEmailLoading = $state(false);
    let sendReportLoading = $state(false);
    let testEmailSuccess = $state(false);
    let reportSuccess = $state(false);

    async function loadSettings(): Promise<void> {
        loading = true;
        error = "";
        try {
            const data = await getSettings();
            globalPublicKey = data.global_ssh_public_key;
            defaultSshUser = data.default_ssh_pull_user || "root";
            askToCopyPublicKey = data.ask_to_copy_public_key;
            if (data.smtp_settings) {
                smtpSettings = { ...data.smtp_settings, password: "" };
            }
            if (data.email_frequency) {
                emailFrequency = data.email_frequency;
            }
        } catch (err) {
            error =
                err instanceof Error ? err.message : "Failed to load settings";
        } finally {
            loading = false;
        }
    }

    async function saveSettings(): Promise<void> {
        saving = true;
        error = "";
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
            error =
                err instanceof Error ? err.message : "Failed to save settings";
        } finally {
            saving = false;
        }
    }

    async function handleTestEmail(): Promise<void> {
        if (!testEmailTo) {
            error = "Test email address is required";
            return;
        }
        testEmailLoading = true;
        error = "";
        testEmailSuccess = false;
        try {
            await testEmail(testEmailTo);
            testEmailSuccess = true;
            setTimeout(() => (testEmailSuccess = false), 3000);
        } catch (err) {
            error = err instanceof Error ? err.message : "Failed to send test email";
        } finally {
            testEmailLoading = false;
        }
    }

    async function handleSendReportNow(): Promise<void> {
        sendReportLoading = true;
        error = "";
        reportSuccess = false;
        try {
            await sendReportNow();
            reportSuccess = true;
            setTimeout(() => (reportSuccess = false), 3000);
        } catch (err) {
            error = err instanceof Error ? err.message : "Failed to send report";
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
    <div class="tabs-container">
        <button
            type="button"
            class="tab-btn"
            class:active={activeTab === "general"}
            onclick={() => (activeTab = "general")}
        >
            General
        </button>
        <button
            type="button"
            class="tab-btn"
            class:active={activeTab === "integrations"}
            onclick={() => (activeTab = "integrations")}
        >
            Integrations
        </button>
        <button
            type="button"
            class="tab-btn"
            class:active={activeTab === "security"}
            onclick={() => (activeTab = "security")}
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
            <button
                type="button"
                class="btn btn-secondary btn-sm"
                onclick={loadSettings}
            >
                Retry
            </button>
        </div>
    {:else}
        {#if activeTab === "general"}
            <div class="settings-card" style="margin-bottom: 24px;">
                <h2 class="settings-title">Default SSH Pull User</h2>
                <p class="settings-description">
                    This user will be pre-filled as the default when registering
                    new SSH pull hosts.
                </p>
                <form
                    onsubmit={(e) => {
                        e.preventDefault();
                        void saveSettings();
                    }}
                >
                    <div style="display: flex; gap: 12px; align-items: center;">
                        <input
                            type="text"
                            class="form-input"
                            style="max-width: 300px;"
                            bind:value={defaultSshUser}
                            placeholder="root"
                            required
                        />
                        <div
                            style="display: flex; gap: 12px; align-items: center; margin-top: 16px;"
                        >
                            <label
                                style="display: flex; align-items: center; gap: 8px; cursor: pointer;"
                            >
                                <input
                                    type="checkbox"
                                    bind:checked={askToCopyPublicKey}
                                    style="width: 16px; height: 16px; accent-color: var(--accent);"
                                />
                                <span
                                    style="font-size: 14px; color: var(--text-primary);"
                                    >Ask to copy public key after creating SSH
                                    host</span
                                >
                            </label>
                        </div>
                        <div style="margin-top: 16px;">
                            <button
                                type="submit"
                                class="btn btn-primary"
                                disabled={saving}
                            >
                                {saving ? "Saving..." : "Save"}
                            </button>
                            {#if saved}
                                <span
                                    style="color: var(--green); font-size: 14px; margin-left: 12px;"
                                    >Saved!</span
                                >
                            {/if}
                        </div>
                    </div>
                </form>
            </div>

            <div class="settings-card">
                <h2 class="settings-title">Global SSH Key</h2>
                <p class="settings-description">
                    This key pair is used globally by default for SSH pull
                    hosts. Add this public key to the <code
                        >~/.ssh/authorized_keys</code
                    > file on your target hosts to authorize PatchBase.
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
                        <button
                            type="button"
                            class="btn btn-primary"
                            onclick={copyKey}
                        >
                            {copied ? "Copied!" : "Copy Public Key"}
                        </button>
                    </div>
                </div>
            </div>
        {:else if activeTab === "integrations"}
            <div class="settings-card" style="margin-bottom: 24px;">
                <h2 class="settings-title">Email Notifications (SMTP)</h2>
                <p class="settings-description">
                    Configure your SMTP server to receive daily patch reports.
                </p>
                <form
                    onsubmit={(e) => {
                        e.preventDefault();
                        void saveSettings();
                    }}
                >
                    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 16px;">
                        <div>
                            <label class="key-label" for="smtp-host">Host</label>
                            <input id="smtp-host" type="text" class="form-input" bind:value={smtpSettings.host} placeholder="smtp.example.com" />
                        </div>
                        <div>
                            <label class="key-label" for="smtp-port">Port</label>
                            <input id="smtp-port" type="number" class="form-input" bind:value={smtpSettings.port} placeholder="587" />
                        </div>
                        <div>
                            <label class="key-label" for="smtp-username">Username</label>
                            <input id="smtp-username" type="text" class="form-input" bind:value={smtpSettings.username} />
                        </div>
                        <div>
                            <label class="key-label" for="smtp-password">Password</label>
                            <input id="smtp-password" type="password" class="form-input" bind:value={smtpSettings.password} placeholder="Leave blank to keep unchanged" />
                        </div>
                        <div style="grid-column: span 2;">
                            <label class="key-label" for="smtp-from">From Address</label>
                            <input id="smtp-from" type="email" class="form-input" bind:value={smtpSettings.from} placeholder="noreply@example.com" />
                        </div>
                        <div style="grid-column: span 2;">
                            <label class="key-label" for="email-frequency">Report Frequency</label>
                            <select id="email-frequency" class="form-input" bind:value={emailFrequency}>
                                <option value="never">Never (Disabled)</option>
                                <option value="daily">Daily</option>
                            </select>
                        </div>
                        {#if emailFrequency === 'daily'}
                            <div style="grid-column: span 2;">
                                <label class="key-label" for="report-hour">Report Hour (UTC)</label>
                                <select id="report-hour" class="form-input" bind:value={smtpSettings.report_hour}>
                                    {#each Array.from({length: 24}, (_, i) => i) as hour}
                                        <option value={hour}>{hour.toString().padStart(2, '0')}:00 UTC</option>
                                    {/each}
                                </select>
                            </div>
                        {/if}
                    </div>
                    <div style="display: flex; gap: 12px; align-items: center; margin-bottom: 24px;">
                        <button type="submit" class="btn btn-primary" disabled={saving}>
                            {saving ? "Saving..." : "Save Settings"}
                        </button>
                        {#if saved}
                            <span style="color: var(--green); font-size: 14px;">Saved!</span>
                        {/if}
                    </div>
                </form>

                <hr style="border: none; border-top: 1px solid var(--border); margin: 24px 0;" />
                
                <h3 class="settings-title" style="font-size: 16px;">Test & Manual Actions</h3>
                <div style="display: flex; flex-direction: column; gap: 16px; margin-top: 16px;">
                    <div style="display: flex; gap: 12px; align-items: flex-end;">
                        <div style="flex: 1;">
                            <label class="key-label" for="test-email">Test Email Address</label>
                            <input id="test-email" type="email" class="form-input" bind:value={testEmailTo} placeholder="admin@example.com" />
                        </div>
                        <button type="button" class="btn btn-secondary" onclick={handleTestEmail} disabled={testEmailLoading}>
                            {testEmailLoading ? "Sending..." : "Send Test Email"}
                        </button>
                        {#if testEmailSuccess}
                            <span style="color: var(--green); font-size: 14px; align-self: center;">Test sent!</span>
                        {/if}
                    </div>

                    <div style="display: flex; gap: 12px; align-items: center; margin-top: 8px;">
                        <button type="button" class="btn btn-secondary" onclick={handleSendReportNow} disabled={sendReportLoading}>
                            {sendReportLoading ? "Sending..." : "Send Report Now"}
                        </button>
                        {#if reportSuccess}
                            <span style="color: var(--green); font-size: 14px;">Report sent!</span>
                        {/if}
                    </div>
                </div>
            </div>
        {:else if activeTab === "security"}
            <div class="settings-card">
                <h2 class="settings-title">Security Settings</h2>
                <p class="settings-description">
                    Manage security levels, access controls, token expiration
                    policies, and logging preferences.
                </p>
                <div>
                    <p style="color: var(--text-dim); font-style: italic;">
                        Access control settings are handled via Identity
                        provider.
                    </p>
                </div>
            </div>
        {/if}
    {/if}
</AppLayout>

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
        content: "";
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
