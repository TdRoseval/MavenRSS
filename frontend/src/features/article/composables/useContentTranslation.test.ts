import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import {
  extractTextWithPlaceholders,
  restorePreservedElements,
  hasOnlyPreservedContent,
  getTranslatableText,
} from './useContentTranslation';

describe('useContentTranslation', () => {
  let container: HTMLElement;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
  });

  afterEach(() => {
    document.body.removeChild(container);
  });

  describe('extractTextWithPlaceholders', () => {
    it('should extract text with inline code placeholders', () => {
      container.innerHTML = '<p>Use the <code>console.log</code> function</p>';
      const p = container.querySelector('p') as HTMLElement;

      const { text, preservedElements, hyperlinks } = extractTextWithPlaceholders(p);

      expect(text).toContain('⟦0⟧');
      expect(preservedElements).toHaveLength(1);
      expect(preservedElements[0].outerHTML).toBe('<code>console.log</code>');
      expect(hyperlinks).toHaveLength(0);
    });

    it('should extract text with multiple inline elements', () => {
      container.innerHTML = '<p>The <code>x</code> equals <code>y</code></p>';
      const p = container.querySelector('p') as HTMLElement;

      const { text, preservedElements } = extractTextWithPlaceholders(p);

      expect(text).toContain('⟦0⟧');
      expect(text).toContain('⟦1⟧');
      expect(preservedElements).toHaveLength(2);
    });

    it('should extract text with image placeholders', () => {
      container.innerHTML = '<p>An image <img src="test.png" alt="test"> here</p>';
      const p = container.querySelector('p') as HTMLElement;

      const { text, preservedElements } = extractTextWithPlaceholders(p);

      expect(text).toContain('⟦0⟧');
      expect(preservedElements).toHaveLength(1);
      expect(preservedElements[0].outerHTML).toContain('<img');
    });

    it('should handle text without preserved elements', () => {
      container.innerHTML = '<p>Simple text without special elements</p>';
      const p = container.querySelector('p') as HTMLElement;

      const { text, preservedElements, hyperlinks } = extractTextWithPlaceholders(p);

      expect(text).toBe('Simple text without special elements');
      expect(preservedElements).toHaveLength(0);
      expect(hyperlinks).toHaveLength(0);
    });

    it('should extract text with kbd elements', () => {
      container.innerHTML = '<p>Press <kbd>Ctrl</kbd>+<kbd>C</kbd> to copy</p>';
      const p = container.querySelector('p') as HTMLElement;

      const { preservedElements } = extractTextWithPlaceholders(p);

      expect(preservedElements).toHaveLength(2);
      expect(preservedElements[0].outerHTML).toBe('<kbd>Ctrl</kbd>');
      expect(preservedElements[1].outerHTML).toBe('<kbd>C</kbd>');
    });

    it('should extract text with sub and sup elements', () => {
      container.innerHTML = '<p>H<sub>2</sub>O and E=mc<sup>2</sup></p>';
      const p = container.querySelector('p') as HTMLElement;

      const { preservedElements } = extractTextWithPlaceholders(p);

      expect(preservedElements).toHaveLength(2);
      expect(preservedElements[0].outerHTML).toBe('<sub>2</sub>');
      expect(preservedElements[1].outerHTML).toBe('<sup>2</sup>');
    });

    it('should handle hyperlinks with markers for translation', () => {
      container.innerHTML = '<p>Visit <a href="https://example.com">this link</a> for more</p>';
      const p = container.querySelector('p') as HTMLElement;

      const { text, preservedElements, hyperlinks } = extractTextWithPlaceholders(p);

      // Hyperlinks should be marked with special markers
      expect(text).toContain('⟪0this link0⟫');
      expect(preservedElements).toHaveLength(0); // Links are not in preserved elements
      expect(hyperlinks).toHaveLength(1);
      expect(hyperlinks[0].href).toBe('https://example.com');
    });

    it('should handle multiple hyperlinks', () => {
      container.innerHTML =
        '<p>Visit <a href="https://example.com">link1</a> and <a href="https://test.com">link2</a></p>';
      const p = container.querySelector('p') as HTMLElement;

      const { text, hyperlinks } = extractTextWithPlaceholders(p);

      expect(text).toContain('⟪0link10⟫');
      expect(text).toContain('⟪1link21⟫');
      expect(hyperlinks).toHaveLength(2);
      expect(hyperlinks[0].href).toBe('https://example.com');
      expect(hyperlinks[1].href).toBe('https://test.com');
    });
  });

  describe('restorePreservedElements', () => {
    it('should restore code elements in translated text', () => {
      const translatedText = 'Use the ⟦0⟧ function';
      const preservedElements = [
        {
          placeholder: '⟦0⟧',
          outerHTML: '<code>console.log</code>',
          element: document.createElement('code'),
        },
      ];

      const result = restorePreservedElements(translatedText, preservedElements);

      expect(result).toContain('<code>console.log</code>');
      expect(result).not.toContain('⟦0⟧');
    });

    it('should restore multiple preserved elements', () => {
      const translatedText = 'The ⟦0⟧ equals ⟦1⟧';
      const preservedElements = [
        {
          placeholder: '⟦0⟧',
          outerHTML: '<code>x</code>',
          element: document.createElement('code'),
        },
        {
          placeholder: '⟦1⟧',
          outerHTML: '<code>y</code>',
          element: document.createElement('code'),
        },
      ];

      const result = restorePreservedElements(translatedText, preservedElements);

      expect(result).toContain('<code>x</code>');
      expect(result).toContain('<code>y</code>');
    });

    it('should escape HTML in translated text but preserve elements', () => {
      const translatedText = 'Use <script> and ⟦0⟧';
      const preservedElements = [
        {
          placeholder: '⟦0⟧',
          outerHTML: '<code>test</code>',
          element: document.createElement('code'),
        },
      ];

      const result = restorePreservedElements(translatedText, preservedElements);

      expect(result).toContain('&lt;script&gt;');
      expect(result).toContain('<code>test</code>');
    });

    it('should handle placeholders with spaces around them', () => {
      const translatedText = 'The ⟦ 0 ⟧ function';
      const preservedElements = [
        {
          placeholder: '⟦0⟧',
          outerHTML: '<code>test</code>',
          element: document.createElement('code'),
        },
      ];

      const result = restorePreservedElements(translatedText, preservedElements);

      expect(result).toContain('<code>test</code>');
    });

    it('should restore hyperlinks in translated text', () => {
      const translatedText = 'Visit ⟪0translated link0⟫ for more';
      const hyperlinks = [
        {
          startMarker: '⟪0',
          endMarker: '0⟫',
          href: 'https://example.com',
          attributes: { href: 'https://example.com', class: 'link' },
        },
      ];

      const result = restorePreservedElements(translatedText, [], hyperlinks);

      expect(result).toContain('<a href="https://example.com" class="link">translated link</a>');
      expect(result).not.toContain('⟪');
      expect(result).not.toContain('⟫');
    });

    it('should restore multiple hyperlinks in translated text', () => {
      const translatedText = 'Visit ⟪0first link0⟫ and ⟪1second link1⟫';
      const hyperlinks = [
        {
          startMarker: '⟪0',
          endMarker: '0⟫',
          href: 'https://example.com',
          attributes: { href: 'https://example.com' },
        },
        {
          startMarker: '⟪1',
          endMarker: '1⟫',
          href: 'https://test.com',
          attributes: { href: 'https://test.com' },
        },
      ];

      const result = restorePreservedElements(translatedText, [], hyperlinks);

      expect(result).toContain('<a href="https://example.com">first link</a>');
      expect(result).toContain('<a href="https://test.com">second link</a>');
    });

    it('should restore both preserved elements and hyperlinks', () => {
      const translatedText = 'Use ⟦0⟧ and visit ⟪0this link0⟫';
      const preservedElements = [
        {
          placeholder: '⟦0⟧',
          outerHTML: '<code>API</code>',
          element: document.createElement('code'),
        },
      ];
      const hyperlinks = [
        {
          startMarker: '⟪0',
          endMarker: '0⟫',
          href: 'https://example.com',
          attributes: { href: 'https://example.com' },
        },
      ];

      const result = restorePreservedElements(translatedText, preservedElements, hyperlinks);

      expect(result).toContain('<code>API</code>');
      expect(result).toContain('<a href="https://example.com">this link</a>');
    });
  });

  describe('hasOnlyPreservedContent', () => {
    it('should return true for element with only code', () => {
      container.innerHTML = '<p><code>console.log("test")</code></p>';
      const p = container.querySelector('p') as HTMLElement;

      expect(hasOnlyPreservedContent(p)).toBe(true);
    });

    it('should return false for element with text and code', () => {
      container.innerHTML = '<p>Use the <code>function</code> to log</p>';
      const p = container.querySelector('p') as HTMLElement;

      expect(hasOnlyPreservedContent(p)).toBe(false);
    });

    it('should return true for element with only image', () => {
      container.innerHTML = '<p><img src="test.png" alt="test"></p>';
      const p = container.querySelector('p') as HTMLElement;

      expect(hasOnlyPreservedContent(p)).toBe(true);
    });

    it('should return false for element with text', () => {
      container.innerHTML = '<p>Some regular text</p>';
      const p = container.querySelector('p') as HTMLElement;

      expect(hasOnlyPreservedContent(p)).toBe(false);
    });
  });

  describe('getTranslatableText', () => {
    it('should return text without code elements', () => {
      container.innerHTML = '<p>Use the <code>function</code> to log</p>';
      const p = container.querySelector('p') as HTMLElement;

      const result = getTranslatableText(p);

      expect(result).toBe('Use the  to log');
    });

    it('should return text without images', () => {
      container.innerHTML = '<p>An image <img src="test.png"> here</p>';
      const p = container.querySelector('p') as HTMLElement;

      const result = getTranslatableText(p);

      expect(result).toBe('An image  here');
    });

    it('should return empty string for element with only preserved content', () => {
      container.innerHTML = '<p><code>code only</code></p>';
      const p = container.querySelector('p') as HTMLElement;

      const result = getTranslatableText(p);

      expect(result).toBe('');
    });
  });
});
