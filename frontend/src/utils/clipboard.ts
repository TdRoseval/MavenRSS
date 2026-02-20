/**
 * Clipboard utilities for MrRSS
 * Uses Wails v3 native Clipboard API with browser fallback
 */

// Try to import Wails runtime, but fallback to browser API if not available
let wailsClipboard: any = null;

// Check if Wails runtime is available
function isWailsAvailable(): boolean {
  return typeof window !== 'undefined' && (window as any).wails !== undefined;
}

// Lazy load Wails clipboard API when needed
async function getWailsClipboard() {
  if (wailsClipboard === null && isWailsAvailable()) {
    try {
      // Try to import @wailsio/runtime
      const { Clipboard } = await import('@wailsio/runtime');
      wailsClipboard = Clipboard;
    } catch (error) {
      console.log('Wails runtime not available, using browser clipboard API');
      wailsClipboard = false;
    }
  }
  return wailsClipboard;
}

/**
 * Copy text to clipboard using Wails v3 native API or browser API as fallback
 * @param text Text to copy
 * @returns Promise that resolves to true if successful, false otherwise
 */
async function copyToClipboard(text: string): Promise<boolean> {
  if (!text) {
    console.warn('copyToClipboard: text is empty');
    return false;
  }

  try {
    // Try Wails native API first
    const clipboard = await getWailsClipboard();
    if (clipboard) {
      await clipboard.SetText(text);
      return true;
    }

    // Fallback to browser clipboard API
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }

    // Fallback to document.execCommand for older browsers
    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.left = '-999999px';
    textArea.style.top = '-999999px';
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    const success = document.execCommand('copy');
    document.body.removeChild(textArea);
    return success;
  } catch (error) {
    console.error('Failed to copy to clipboard:', error);
    return false;
  }
}

/**
 * Copy article URL to clipboard
 * @param url Article URL
 * @returns Promise that resolves to true if successful, false otherwise
 */
export async function copyArticleLink(url: string): Promise<boolean> {
  return copyToClipboard(url);
}

/**
 * Copy article title to clipboard
 * @param title Article title
 * @returns Promise that resolves to true if successful, false otherwise
 */
export async function copyArticleTitle(title: string): Promise<boolean> {
  return copyToClipboard(title);
}

/**
 * Copy feed URL to clipboard
 * @param url Feed URL
 * @returns Promise that resolves to true if successful, false otherwise
 */
export async function copyFeedURL(url: string): Promise<boolean> {
  return copyToClipboard(url);
}
