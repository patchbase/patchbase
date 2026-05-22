<script lang="ts">
	import { deleteHost } from '$lib/api/hosts.js';
	import type { Host } from '$lib/types';

	interface Props {
		host: Host;
		class?: string;
		buttonText?: string;
		onDelete?: () => void;
		onError?: (err: Error) => void;
	}

	let { host, class: className = '', buttonText = 'Delete host', onDelete, onError }: Props = $props();

	let deleting = $state(false);

	function hostLabel(): string {
		return host.display_name || host.hostname || host.id;
	}

	async function handleDelete() {
		const label = hostLabel();
		if (!window.confirm(`Delete host "${label}"? This will remove snapshots and access tokens.`)) {
			return;
		}

		deleting = true;
		try {
			await deleteHost(host.id);
			if (onDelete) onDelete();
		} catch (err) {
			if (onError) onError(err instanceof Error ? err : new Error(String(err)));
		} finally {
			deleting = false;
		}
	}
</script>

<button
	type="button"
	class={className}
	onclick={() => void handleDelete()}
	disabled={deleting}
>
	{deleting ? 'Deleting...' : buttonText}
</button>
