/**
 * Date formatting utilities for MrRSS
 */

/**
 * Format a date string for display in the current locale
 * @param dateStr - ISO date string
 * @param locale - Locale code (e.g., 'en-US', 'zh-CN')
 * @returns Formatted date string
 */
export function formatDate(dateStr: string, locale: string = 'en-US'): string {
  if (!dateStr) return '';
  try {
    return new Date(dateStr).toLocaleDateString(locale);
  } catch {
    return '';
  }
}

/**
 * Format a timestamp as a relative time (e.g., "2 hours ago")
 * @param timestamp - ISO timestamp string
 * @param locale - Current locale for translations
 * @param t - Translation function from i18n
 * @returns Formatted relative time string
 */
export function formatRelativeTime(
  timestamp: string,
  locale: string,
  t: (key: string, params?: Record<string, unknown>) => string
): string {
  if (!timestamp) return t('never');
  try {
    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return t('justNow');
    if (diffMins < 60) return t('minutesAgo', { count: diffMins });
    if (diffHours < 24) return t('hoursAgo', { count: diffHours });
    if (diffDays < 7) return t('daysAgo', { count: diffDays });

    // Get locale for date formatting
    return date.toLocaleDateString(locale === 'zh-CN' ? 'zh-CN' : 'en-US');
  } catch {
    return t('never');
  }
}
