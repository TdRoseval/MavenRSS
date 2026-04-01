import type { Feed } from '@/types/models';

const IOWEN_FAVICON_HOST = 'api.iowen.cn';
const FALLBACK_COLORS = ['#2563EB', '#0F766E', '#EA580C', '#7C3AED', '#BE185D', '#4F46E5'];

function safeDecodeURL(url: string): string {
  try {
    return decodeURIComponent(url);
  } catch {
    return url;
  }
}

function isLegacyExternalFavicon(url?: string): boolean {
  if (!url) return false;

  try {
    const parsed = new URL(safeDecodeURL(url));
    return parsed.hostname === IOWEN_FAVICON_HOST && parsed.pathname.startsWith('/favicon/');
  } catch {
    return false;
  }
}

function pickInitial(feed: Pick<Feed, 'title' | 'url'>): string {
  const titleInitial = feed.title?.trim().charAt(0);
  if (titleInitial) {
    return titleInitial.toUpperCase();
  }

  try {
    const hostname = new URL(feed.url).hostname.replace(/^www\./i, '');
    return hostname.charAt(0).toUpperCase() || 'R';
  } catch {
    return 'R';
  }
}

function pickBackground(seed: string): string {
  let hash = 0;
  for (let i = 0; i < seed.length; i += 1) {
    hash = (hash * 31 + seed.charCodeAt(i)) >>> 0;
  }

  return FALLBACK_COLORS[hash % FALLBACK_COLORS.length];
}

function buildFallbackIconDataURL(feed: Pick<Feed, 'title' | 'url'>): string {
  const initial = pickInitial(feed);
  const background = pickBackground(feed.url || feed.title || initial);
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">
      <rect width="64" height="64" rx="14" fill="${background}" />
      <text x="50%" y="50%" dominant-baseline="central" text-anchor="middle"
        font-family="Segoe UI, Arial, sans-serif" font-size="28" font-weight="700" fill="#FFFFFF">
        ${initial}
      </text>
    </svg>
  `.trim();

  return `data:image/svg+xml;charset=UTF-8,${encodeURIComponent(svg)}`;
}

export function getFeedDisplayIconUrl(feed: Pick<Feed, 'title' | 'url' | 'image_url'>): string {
  if (feed.image_url && !isLegacyExternalFavicon(feed.image_url)) {
    return feed.image_url;
  }

  return buildFallbackIconDataURL(feed);
}
