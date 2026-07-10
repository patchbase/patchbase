export function createPollingFallback(fetchFn: () => Promise<void>, intervalMs: number = 5000) {
  let timer: ReturnType<typeof setInterval> | null = null;

  return {
    start: () => {
      if (!timer) {
        timer = setInterval(() => {
          void fetchFn();
        }, intervalMs);
      }
    },
    stop: () => {
      if (timer) {
        clearInterval(timer);
        timer = null;
      }
    },
  };
}
