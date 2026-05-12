<script lang="ts">
	import { goto } from '$app/navigation';
	import '../app.css';
	import { getSetupStatus } from '$lib/api/auth.js';
	import { getSession } from '$lib/auth/session.js';
	import { onMount } from 'svelte';
	import type { Snippet } from 'svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();
	let loading = $state(true);
	let setupCompleted = $state<boolean | null>(null);

	const setupStatusPollInterval = 5000;
	const sessionChangedEvent = 'patchbase:session-changed';

	function normalizePath(value: string): string {
		if (value.length > 1 && value.endsWith('/')) {
			return value.slice(0, -1);
		}
		return value;
	}

	function isLoginRoute(pathname: string): boolean {
		return pathname === '/login';
	}

	function isSetupRoute(pathname: string): boolean {
		return pathname === '/setup';
	}

	async function refreshSetupStatus(): Promise<void> {
		try {
			const result = await getSetupStatus();
			setupCompleted = result.completed;
		} catch {
			setupCompleted = true;
		}
	}

	async function enforceRouteGuard(): Promise<void> {
		const pathname = normalizePath(window.location.pathname);
		const session = getSession();
		const isLoggedIn = !!session?.accessToken;

		if (!isLoggedIn) {
			if (!isLoginRoute(pathname)) {
				await goto('/login', { replaceState: true });
			}
			return;
		}

		if (setupCompleted === false || session.passwordResetNeeded) {
			if (!isSetupRoute(pathname)) {
				await goto('/setup', { replaceState: true });
			}
			return;
		}

		if (isLoginRoute(pathname) || isSetupRoute(pathname)) {
			await goto('/', { replaceState: true });
		}
	}

	onMount(() => {
		let unmounted = false;
		let pollTimer: ReturnType<typeof setInterval> | undefined;

		const runGuardCycle = async (): Promise<void> => {
			await refreshSetupStatus();
			if (unmounted) {
				return;
			}
			if (loading) {
				loading = false;
			}
			await enforceRouteGuard();
		};

		const run = (): void => {
			void runGuardCycle();
		};

		void runGuardCycle();
		pollTimer = setInterval(run, setupStatusPollInterval);

		const onSessionChanged = (): void => {
			void enforceRouteGuard();
		};
		const onPopState = (): void => {
			void enforceRouteGuard();
		};

		window.addEventListener(sessionChangedEvent, onSessionChanged);
		window.addEventListener('popstate', onPopState);

		return () => {
			unmounted = true;
			if (pollTimer) {
				clearInterval(pollTimer);
			}
			window.removeEventListener(sessionChangedEvent, onSessionChanged);
			window.removeEventListener('popstate', onPopState);
		};
	});
</script>

{#if loading}
	<div class="auth-page">
		<div class="auth-card">
			<h2>Loading</h2>
			<p class="auth-subtitle">Checking setup and session state.</p>
		</div>
	</div>
{:else}
	{@render children()}
{/if}
