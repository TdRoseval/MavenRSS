let isServerModeCache: boolean | null = null;
let serverModeCheckPromise: Promise<boolean> | null = null;

export async function checkServerMode(): Promise<boolean> {
  if (isServerModeCache !== null) {
    return isServerModeCache;
  }

  if (serverModeCheckPromise) {
    return serverModeCheckPromise;
  }

  serverModeCheckPromise = (async () => {
    try {
      const res = await fetch('/api/version');
      if (res.ok) {
        const data = await res.json();
        isServerModeCache = data.server_mode === 'true';
        return isServerModeCache;
      }
    } catch (e) {
      console.error('Error checking server mode:', e);
    }
    isServerModeCache = false;
    return false;
  })();

  return serverModeCheckPromise;
}

export function getCachedServerMode(): boolean | null {
  return isServerModeCache;
}

export function setCachedServerMode(isServer: boolean): void {
  isServerModeCache = isServer;
}
