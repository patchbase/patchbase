<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		open?: boolean;
		title: string;
		children: Snippet;
		footer?: Snippet;
		onclose?: () => void;
		dismissible?: boolean;
	}

	let { open = false, title, children, footer, onclose, dismissible = true }: Props = $props();

	let dialog: HTMLDialogElement | undefined = undefined;

	$effect(() => {
		if (dialog) {
			if (open && !dialog.open) {
				dialog.showModal();
			} else if (!open && dialog.open) {
				dialog.close();
			}
		}
	});

	function handleCancel(e: Event) {
		if (!dismissible) {
			e.preventDefault();
		} else if (onclose) {
			onclose();
		}
	}
</script>

<dialog
	bind:this={dialog}
	oncancel={handleCancel}
	class="modal-dialog"
>
	<div class="modal-content">
		<div class="modal-header">
			<h3>{title}</h3>
			{#if dismissible}
				<button class="modal-close" type="button" aria-label="Close" onclick={() => onclose && onclose()}>&times;</button>
			{/if}
		</div>
		<div class="modal-body">
			{@render children()}
		</div>
		{#if footer}
			<div class="modal-footer">
				{@render footer()}
			</div>
		{/if}
	</div>
</dialog>

<style>
	.modal-dialog {
		margin: auto;
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 0;
		background: var(--bg-card);
		color: var(--text-primary);
		box-shadow: 0 8px 24px rgba(0,0,0,0.15);
		max-width: 500px;
		width: 100%;
	}
	.modal-dialog::backdrop {
		background: rgba(0, 0, 0, 0.5);
		backdrop-filter: blur(2px);
	}
	.modal-content {
		display: flex;
		flex-direction: column;
	}
	.modal-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 16px 20px;
		border-bottom: 1px solid var(--border);
	}
	.modal-header h3 {
		margin: 0;
		font-size: 1.25rem;
		font-weight: 600;
	}
	.modal-close {
		background: none;
		border: none;
		font-size: 1.5rem;
		line-height: 1;
		cursor: pointer;
		color: var(--text-muted);
	}
	.modal-close:hover {
		color: var(--text-primary);
	}
	.modal-body {
		padding: 20px;
	}
	.modal-footer {
		padding: 16px 20px;
		border-top: 1px solid var(--border);
		display: flex;
		justify-content: flex-end;
		gap: 12px;
	}
</style>
