<script lang="ts">
	// SPDX-FileCopyrightText: 2026 Configure Labs SRL
	// SPDX-License-Identifier: AGPL-3.0-only
    import { approveHost } from "$lib/api/hosts.js";
    import type { Host } from "$lib/types";

    interface Props {
        host: Host;
        class?: string;
        onApprove?: () => void;
        onError?: (err: Error) => void;
    }

    let { host, class: className = "", onApprove, onError }: Props = $props();

    let approving = $state(false);

    async function handleApprove() {
        approving = true;
        try {
            await approveHost(host.id);
            if (onApprove) onApprove();
        } catch (err) {
            if (onError)
                onError(err instanceof Error ? err : new Error(String(err)));
        } finally {
            approving = false;
        }
    }
</script>

{#if host.approval_status === "waiting_approval"}
    <button
        type="button"
        class={className}
        onclick={() => void handleApprove()}
        disabled={approving}
    >
        {approving ? "Approving..." : "Approve host"}
    </button>
{/if}
