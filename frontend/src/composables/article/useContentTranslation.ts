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
  // Hyperlinks (anchor tags)
  'a',
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

interface PreservedElement {
  placeholder: string;
  outerHTML: string;
  element: Element;
}

/**
 * Extract text for translation while preserving inline elements
 * Returns the text with placeholders and a map to restore elements
 */
export function extractTextWithPlaceholders(element: HTMLElement): {
  text: string;
  preservedElements: PreservedElement[];
} {
  const preservedElements: PreservedElement[] = [];

  // Clone the element to avoid modifying the original
  const clone = element.cloneNode(true) as HTMLElement;

  // Find all elements that should be preserved
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

  // Get the text content with placeholders
  const text = clone.textContent?.trim() || '';

  return { text, preservedElements };
}

/**
 * Restore preserved elements in the translated text
 * Returns HTML string with preserved elements restored
 */
export function restorePreservedElements(
  translatedText: string,
  preservedElements: PreservedElement[]
): string {
  let result = translatedText;

  // Escape HTML in the translated text (except placeholders)
  result = result
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');

  // Restore each preserved element
  for (const { placeholder, outerHTML } of preservedElements) {
    // The placeholder might have been slightly modified by translation
    // Try exact match first
    if (result.includes(placeholder)) {
      result = result.replace(placeholder, outerHTML);
    } else {
      // Try matching with possible spaces or modifications
      const escapedPrefix = PLACEHOLDER_PREFIX.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
      const escapedSuffix = PLACEHOLDER_SUFFIX.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
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

  // Check if there's any meaningful text left
  const remainingText = clone.textContent?.trim() || '';
  return remainingText.length < 2;
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
