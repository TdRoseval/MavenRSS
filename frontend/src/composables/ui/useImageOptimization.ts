import { ref, onMounted, onUnmounted } from 'vue';

export function useImageOptimization() {
  const isIntersectionObserverSupported = typeof IntersectionObserver !== 'undefined';
  const loadedImages = new Set<string>();

  const loadImage = (src: string): Promise<string> => {
    return new Promise((resolve, reject) => {
      if (loadedImages.has(src)) {
        resolve(src);
        return;
      }

      const img = new Image();
      img.onload = () => {
        loadedImages.add(src);
        resolve(src);
      };
      img.onerror = reject;
      img.src = src;
    });
  };

  const observeImage = (element: HTMLElement, src: string, callback: (src: string) => void) => {
    if (!isIntersectionObserverSupported) {
      loadImage(src).then(callback);
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            loadImage(src).then(callback);
            observer.unobserve(element);
          }
        });
      },
      { rootMargin: '100px 0px' }
    );

    observer.observe(element);
  };

  const getOptimizedImageUrl = (url: string, options?: { width?: number; height?: number; quality?: number }): string => {
    return url;
  };

  return {
    loadImage,
    observeImage,
    getOptimizedImageUrl,
    isIntersectionObserverSupported
  };
}
