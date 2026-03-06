/**
 * Content translation utilities that preserve inline elements
 * (formulas, code, images, etc.) during translation.
 */

/**
 * Inline element types that should be preserved during translation
 */
const PRESERVED_SELECTORS = [
  // Math formulas (KaTeX)
  '.katex',
  '.katex-display',
  '.katex-inline',
  '.math',
  '.MathJax',
  // Code elements
  'code',
  'kbd',
  'samp',
  'var',
  // Images and media
  'img',
  'svg',
  'picture',
  'video',
  'audio',
  'canvas',
  // Special inline elements
  'sub',
  'sup',
  'abbr[title]',
  // Preserved by data attribute
  '[data-no-translate]',
];

/**
 * Placeholder format for preserved elements
 * Using a format that's unlikely to be translated
 */
const PLACEHOLDER_PREFIX = '⟦';
const PLACEHOLDER_SUFFIX = '⟧';

/**
 * Special markers for hyperlinks (different from preserved elements)
 * These help us track link boundaries in translated text
 */
const LINK_START_PREFIX = '⟪';
const LINK_END_SUFFIX = '⟫';

/**
 * Hyperlink map delimiters (used internally during restoration)
 */
const HYPERLINK_MAP_START = '__HYPERLINK_MAP__';
const HYPERLINK_MAP_END = '__HYPERLINK_MAP_END__';

interface PreservedElement {
  placeholder: string;
  outerHTML: string;
  element: Element;
}

interface HyperlinkInfo {
  startMarker: string;
  endMarker: string;
  href: string;
  attributes: Record<string, string>;
}

/**
 * Escape regex special characters in a string
 */
function escapeRegexSpecialChars(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Properly escape HTML attribute value
 */
function escapeAttributeValue(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

/**
 * Extract text for translation while preserving inline elements
 * Returns the text with placeholders and a map to restore elements
 */
export function extractTextWithPlaceholders(element: HTMLElement): {
  text: string;
  preservedElements: PreservedElement[];
  hyperlinks: HyperlinkInfo[];
} {
  const preservedElements: PreservedElement[] = [];
  const hyperlinks: HyperlinkInfo[] = [];

  // Clone the element to avoid modifying the original
  const clone = element.cloneNode(true) as HTMLElement;

  // First, handle hyperlinks specially
  let linkIndex = 0;
  const links = clone.querySelectorAll('a');
  links.forEach((link) => {
    const startMarker = `${LINK_START_PREFIX}${linkIndex}`;
    const endMarker = `${linkIndex}${LINK_END_SUFFIX}`;

    // Store hyperlink information
    const attributes: Record<string, string> = {};
    Array.from(link.attributes).forEach((attr) => {
      attributes[attr.name] = attr.value;
    });

    hyperlinks.push({
      startMarker,
      endMarker,
      href: link.getAttribute('href') || '',
      attributes,
    });

    // Replace the link with markers around its text content
    const textContent = link.textContent || '';
    const markedText = document.createTextNode(`${startMarker}${textContent}${endMarker}`);
    link.parentNode?.replaceChild(markedText, link);

    linkIndex++;
  });

  // Then handle other preserved elements (excluding hyperlinks)
  const elementsToPreserve = clone.querySelectorAll(PRESERVED_SELECTORS.join(','));

  let index = 0;
  elementsToPreserve.forEach((el) => {
    // Skip if this element is nested inside another preserved element
    if (
      el.closest(PRESERVED_SELECTORS.filter((s) => !el.matches(s)).join(',') || 'body') !== clone
    ) {
      const parent = el.parentElement;
      if (parent && PRESERVED_SELECTORS.some((sel) => parent.matches(sel))) {
        return;
      }
    }

    const placeholder = `${PLACEHOLDER_PREFIX}${index}${PLACEHOLDER_SUFFIX}`;
    const originalElement = element.querySelectorAll(PRESERVED_SELECTORS.join(','))[index];

    preservedElements.push({
      placeholder,
      outerHTML: el.outerHTML,
      element: originalElement || el,
    });

    // Replace with placeholder text
    const placeholderText = document.createTextNode(placeholder);
    el.parentNode?.replaceChild(placeholderText, el);

    index++;
  });

  // Get the text content with placeholders and link markers
  const text = clone.textContent?.trim() || '';

  return { text, preservedElements, hyperlinks };
}

/**
 * Restore preserved elements and hyperlinks in the translated text
 * Returns HTML string with preserved elements and hyperlinks restored
 */
export function restorePreservedElements(
  translatedText: string,
  preservedElements: PreservedElement[],
  hyperlinks: HyperlinkInfo[] = []
): string {
  let result = translatedText;

  // First, restore hyperlinks by converting markers to HTML
  for (let i = 0; i < hyperlinks.length; i++) {
    const { startMarker, endMarker, attributes } = hyperlinks[i];

    // Find the link text between markers
    const startPattern = escapeRegexSpecialChars(startMarker);
    const endPattern = escapeRegexSpecialChars(endMarker);
    const linkRegex = new RegExp(`${startPattern}(.*?)${endPattern}`, 's');

    const match = result.match(linkRegex);
    if (match) {
      const linkText = match[1];
      // Use a unique placeholder that won't be escaped
      const uniqueId = `__HYPERLINK_${i}__`;
      result = result.replace(linkRegex, uniqueId);

      // Store the link HTML for later restoration
      if (!result.includes(HYPERLINK_MAP_START)) {
        result = HYPERLINK_MAP_START + JSON.stringify({}) + HYPERLINK_MAP_END + result;
      }

      // Build attribute string with proper escaping
      const attrString = Object.entries(attributes)
        .map(([key, value]) => `${key}="${escapeAttributeValue(value)}"`)
        .join(' ');
      const linkHTML = `<a ${attrString}>${linkText}</a>`;

      // Store in map
      const mapRegex = new RegExp(
        `${escapeRegexSpecialChars(HYPERLINK_MAP_START)}(.*?)${escapeRegexSpecialChars(HYPERLINK_MAP_END)}`
      );
      const mapMatch = result.match(mapRegex);
      if (mapMatch) {
        const map = JSON.parse(mapMatch[1]);
        map[uniqueId] = linkHTML;
        result = result.replace(
          mapRegex,
          `${HYPERLINK_MAP_START}${JSON.stringify(map)}${HYPERLINK_MAP_END}`
        );
      }
    }
  }

  // Extract hyperlink map if it exists
  const mapRegex = new RegExp(
    `${escapeRegexSpecialChars(HYPERLINK_MAP_START)}(.*?)${escapeRegexSpecialChars(HYPERLINK_MAP_END)}`
  );
  const mapMatch = result.match(mapRegex);
  let hyperlinkMap: Record<string, string> = {};
  if (mapMatch) {
    hyperlinkMap = JSON.parse(mapMatch[1]);
    result = result.replace(mapRegex, '');
  }

  // Escape HTML in the translated text (except our placeholders)
  result = result
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');

  // Restore hyperlinks from map
  for (const [placeholder, linkHTML] of Object.entries(hyperlinkMap)) {
    result = result.replace(placeholder, linkHTML);
  }

  // Then restore other preserved elements
  for (const { placeholder, outerHTML } of preservedElements) {
    // The placeholder might have been slightly modified by translation
    // Try exact match first
    if (result.includes(placeholder)) {
      result = result.replace(placeholder, outerHTML);
    } else {
      // Try matching with possible spaces or modifications
      const escapedPrefix = escapeRegexSpecialChars(PLACEHOLDER_PREFIX);
      const escapedSuffix = escapeRegexSpecialChars(PLACEHOLDER_SUFFIX);
      const index = placeholder.slice(1, -1);
      const regex = new RegExp(`${escapedPrefix}\\s*${index}\\s*${escapedSuffix}`, 'g');
      result = result.replace(regex, outerHTML);
    }
  }

  return result;
}

/**
 * Check if an element contains only preserved elements (no translatable text)
 */
export function hasOnlyPreservedContent(element: HTMLElement): boolean {
  const clone = element.cloneNode(true) as HTMLElement;

  // Remove all preserved elements
  const elementsToRemove = clone.querySelectorAll(PRESERVED_SELECTORS.join(','));
  elementsToRemove.forEach((el) => el.remove());

  // Note: We do NOT remove hyperlinks because they contain translatable text
  // Hyperlinks should be translated along with their text content

  // Check if there's any meaningful text left - 放宽条件，只在真正完全没有文本时才返回true
  const remainingText = clone.textContent?.trim() || '';
  return remainingText.length === 0;
}

/**
 * Get the text content excluding preserved elements
 * Used to check if there's actually translatable content
 */
export function getTranslatableText(element: HTMLElement): string {
  const clone = element.cloneNode(true) as HTMLElement;

  // Remove all preserved elements
  const elementsToRemove = clone.querySelectorAll(PRESERVED_SELECTORS.join(','));
  elementsToRemove.forEach((el) => el.remove());

  // Note: We do NOT remove hyperlinks because they contain translatable text
  // Hyperlinks should be translated along with their text content

  return clone.textContent?.trim() || '';
}

/**
 * Create a translation element with preserved inline elements
 */
export function createTranslationElement(translatedHTML: string, className: string): HTMLElement {
  const translationEl = document.createElement('div');
  translationEl.className = className;
  translationEl.innerHTML = translatedHTML;
  return translationEl;
}
