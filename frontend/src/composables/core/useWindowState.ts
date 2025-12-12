import { onMounted, onUnmounted } from 'vue';
import {
  WindowGetPosition,
  WindowGetSize,
  WindowSetPosition,
  WindowSetSize,
  WindowIsMaximised,
  WindowMaximise,
} from '../../wailsjs/wailsjs/runtime/runtime';

interface WindowState {
  x: number;
  y: number;
  width: number;
  height: number;
  maximized: boolean;
}

export function useWindowState() {
  let saveTimeout: NodeJS.Timeout | null = null;
  let isRestoringState = false;

  /**
   * Load and restore window state from database
   */
  async function restoreWindowState() {
    try {
      isRestoringState = true;

      const response = await fetch('/api/window/state');
      if (!response.ok) {
        console.warn('Failed to load window state');
        return;
      }

      const data = await response.json();

      // Parse values with defaults
      const x = parseInt(data.x || '0');
      const y = parseInt(data.y || '0');
      const width = parseInt(data.width || '1024');
      const height = parseInt(data.height || '768');
      const maximized = data.maximized === 'true';

      // Only restore if we have valid saved values (not defaults)
      // Check for actual saved values by looking at the raw data
      const hasValidState = data.x && data.y && data.width && data.height;

      if (hasValidState) {
        // Validate size bounds (minimum and reasonable maximum)
        const validWidth = Math.max(400, Math.min(width, 3840));
        const validHeight = Math.max(300, Math.min(height, 2160));

        // Validate position (ensure window is at least partially visible)
        // Allow negative values for multi-monitor setups
        const validX = Math.max(-1000, Math.min(x, 3000));
        const validY = Math.max(-1000, Math.min(y, 3000));

        // First set size and position
        await WindowSetSize(validWidth, validHeight);
        await WindowSetPosition(validX, validY);

        // Then handle maximized state
        if (maximized) {
          await WindowMaximise();
        }

        console.log('Window state restored:', {
          x: validX,
          y: validY,
          width: validWidth,
          height: validHeight,
          maximized,
        });
      } else {
        console.log('No valid window state found, using defaults');
      }
    } catch (error) {
      console.error('Error restoring window state:', error);
    } finally {
      // Wait a bit before allowing saves to prevent immediately overwriting restored state
      setTimeout(() => {
        isRestoringState = false;
      }, 1000);
    }
  }

  /**
   * Save current window state to database
   */
  async function saveWindowState() {
    // Don't save while we're restoring state
    if (isRestoringState) {
      return;
    }

    try {
      const [position, size, maximized] = await Promise.all([
        WindowGetPosition(),
        WindowGetSize(),
        WindowIsMaximised(),
      ]);

      const state: WindowState = {
        x: position.x,
        y: position.y,
        width: size.w,
        height: size.h,
        maximized,
      };

      await fetch('/api/window/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(state),
      });

      console.log('Window state saved:', state);
    } catch (error) {
      console.error('Error saving window state:', error);
    }
  }

  /**
   * Debounced save to avoid excessive writes
   */
  function debouncedSave() {
    if (saveTimeout) {
      clearTimeout(saveTimeout);
    }
    saveTimeout = setTimeout(saveWindowState, 500);
  }

  /**
   * Setup window event listeners
   */
  function setupListeners() {
    // Listen to window resize and move events
    // We use multiple approaches to catch window state changes:

    // 1. Browser resize event (fires when window size changes)
    const handleResize = () => {
      debouncedSave();
    };
    window.addEventListener('resize', handleResize);

    // 2. Visibility change (fires when window is minimized/maximized)
    const handleVisibilityChange = () => {
      if (!document.hidden) {
        debouncedSave();
      }
    };
    document.addEventListener('visibilitychange', handleVisibilityChange);

    // 3. Periodic check as fallback for position changes
    // (position changes don't trigger browser events)
    const checkInterval = setInterval(() => {
      debouncedSave();
    }, 2000); // Check every 2 seconds

    return () => {
      window.removeEventListener('resize', handleResize);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
      clearInterval(checkInterval);
      if (saveTimeout) {
        clearTimeout(saveTimeout);
      }
    };
  }

  /**
   * Initialize window state management
   */
  function init() {
    let cleanup: (() => void) | null = null;

    onMounted(async () => {
      // Wait a bit for the window to be ready
      await new Promise((resolve) => setTimeout(resolve, 500));

      // Restore saved state
      await restoreWindowState();

      // Setup listeners for future changes
      cleanup = setupListeners();

      // Save state on beforeunload
      window.addEventListener('beforeunload', saveWindowState);
    });

    onUnmounted(() => {
      window.removeEventListener('beforeunload', saveWindowState);
      if (cleanup) {
        cleanup();
      }
    });
  }

  return {
    init,
    restoreWindowState,
    saveWindowState,
  };
}
