import { ref, watch, onUnmounted } from 'vue';
import { useClusterStore } from '@/stores/cluster';
import { apiClient } from '@/shared/lib/apiClient';

/**
 * Composable for tracking reading time on clusters in AI Enhanced Mode.
 *
 * Rules:
 * - Starts timer when currentClusterId changes to a valid cluster.
 * - Pauses when document.visibilityState === 'hidden' (tab switch).
 * - If tab is away for > 60s, stops tracking entirely for that cluster.
 * - On cluster switch or scope dispose, reports accumulated time via POST /api/clusters/read-time.
 */
export function useReadingTimeTracker() {
  const clusterStore = useClusterStore();

  const currentTrackingId = ref<number | null>(null);
  const accumulatedMs = ref(0);
  const isTracking = ref(false);

  let intervalId: ReturnType<typeof setInterval> | null = null;
  let pausedAt: number | null = null;

  const TICK_INTERVAL = 1000; // 1 second
  const MAX_AWAY_MS = 60_000; // 1 minute

  function startTracking(clusterId: number): void {
    stopTracking(); // Flush previous if any

    currentTrackingId.value = clusterId;
    accumulatedMs.value = 0;
    isTracking.value = true;
    pausedAt = null;

    intervalId = setInterval(() => {
      if (isTracking.value) {
        accumulatedMs.value += TICK_INTERVAL;
      }
    }, TICK_INTERVAL);
  }

  function pauseTracking(): void {
    if (!isTracking.value) return;
    isTracking.value = false;
    pausedAt = Date.now();
  }

  function resumeTracking(): void {
    if (currentTrackingId.value === null || pausedAt === null) return;

    const awayMs = Date.now() - pausedAt;
    if (awayMs > MAX_AWAY_MS) {
      // Away too long — stop tracking entirely, do NOT report
      cancelTracking();
      return;
    }

    // Resume within 1 minute — continue accumulating
    isTracking.value = true;
    pausedAt = null;
  }

  function cancelTracking(): void {
    if (intervalId !== null) {
      clearInterval(intervalId);
      intervalId = null;
    }
    currentTrackingId.value = null;
    accumulatedMs.value = 0;
    isTracking.value = false;
    pausedAt = null;
  }

  function stopTracking(): void {
    if (intervalId !== null) {
      clearInterval(intervalId);
      intervalId = null;
    }

    const clusterId = currentTrackingId.value;
    const seconds = Math.floor(accumulatedMs.value / 1000);

    // Report if we tracked at least 1 second
    if (clusterId !== null && seconds > 0) {
      apiClient
        .post('/clusters/read-time', {
          cluster_id: clusterId,
          read_time_seconds: seconds,
        })
        .catch((e: unknown) => {
          console.error('Failed to report reading time:', e);
        });
    }

    currentTrackingId.value = null;
    accumulatedMs.value = 0;
    isTracking.value = false;
    pausedAt = null;
  }

  // Handle visibility change (tab switching)
  function handleVisibilityChange(): void {
    if (document.visibilityState === 'hidden') {
      pauseTracking();
    } else {
      resumeTracking();
    }
  }

  document.addEventListener('visibilitychange', handleVisibilityChange);

  // Watch for cluster changes
  watch(
    () => clusterStore.currentClusterId,
    (newId: number | null, oldId: number | null | undefined) => {
      if (oldId != null && oldId !== newId) {
        stopTracking(); // Report previous
      }
      if (newId != null) {
        startTracking(newId);
      }
    }
  );

  // Cleanup on component unmount
  onUnmounted(() => {
    stopTracking();
    document.removeEventListener('visibilitychange', handleVisibilityChange);
  });

  return {
    accumulatedMs,
    isTracking,
    currentTrackingId,
  };
}
