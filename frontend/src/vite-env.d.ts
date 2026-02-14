/// <reference types="vite/client" />

// Vue module declarations
declare module 'vue' {
  export function ref<T>(value: T): { value: T };
  export function computed<T>(getter: () => T): { value: T };
  export function onMounted(callback: () => void): void;
  export function onUnmounted(callback: () => void): void;
  export function watch<T>(source: any, callback: (newValue: T, oldValue: T) => void): void;
  export function watchEffect(callback: () => void): void;
  export function nextTick(callback?: () => void): Promise<void>;
  export function defineComponent(options: any): any;
  export function h(type: any, props: any, children: any): any;
  export function useRouter(): any;
  export function useRoute(): any;
  export function useStore(): any;
  export function useContext(): any;
  export function provide<T>(key: string | symbol, value: T): void;
  export function inject<T>(key: string | symbol, defaultValue?: T): T | undefined;
  export function createApp(options: any): any;
  export function createRouter(options: any): any;
  export function createWebHistory(): any;
  export function createWebHashHistory(): any;
  export function createMemoryHistory(): any;
  export function useMeta(): any;
  export function useHead(): any;
  export function useI18n(): any;
  export function usePreferredColorScheme(): any;
  export function useMediaQuery(): any;
  export function useResizeObserver(): any;
  export function useIntersectionObserver(): any;
  export function useMouse(): any;
  export function useScroll(): any;
  export function useStorage(): any;
  export function useLocalStorage(): any;
  export function useSessionStorage(): any;
  export function useCounter(): any;
  export function useNow(): any;
  export function useTimeAgo(): any;
  export function useDateFormat(): any;
  export function useDebounce(): any;
  export function useThrottle(): any;
  export function useAsyncState(): any;
  export function useFetch(): any;
  export function useAxios(): any;
  export function useWebSocket(): any;
  export function useBreakpoints(): any;
  export function useRouteHash(): any;
  export function useRouteQuery(): any;
  export function useRouteParams(): any;
  export function useRouterPush(): any;
  export function useRouterReplace(): any;
  export function useRouterBack(): any;
  export function useRouterForward(): any;
  export function useRouterGo(): any;
  export function useRouterIsActive(): any;
  export function useRouterLink(): any;
  export function useRouterResolved(): any;
  export function useRouterReady(): any;
  export function useRouterError(): any;
  export function useRouterLoading(): any;
  export function useRouterMeta(): any;
  export function useRouterScroll(): any;
  export function useRouterTitle(): any;
  export function useRouterTransitions(): any;
  export function useRouterView(): any;
  export function useRouterViews(): any;
  export function useRouterMatch(): any;
  export function useRouterGuard(): any;
  export function useRouterBeforeEach(): any;
  export function useRouterAfterEach(): any;
  export function useRouterOnError(): any;
  export function useRouterOnReady(): any;
  export function useRouterOnComplete(): any;
  export function useRouterOnBeforeRouteLeave(): any;
  export function useRouterOnBeforeRouteUpdate(): any;
  export function useRouterOnBeforeRouteEnter(): any;
  export function useRouterOnRouteLeave(): any;
  export function useRouterOnRouteUpdate(): any;
  export function useRouterOnRouteEnter(): any;
  export function useRouterOnRouteChange(): any;
  export function useRouterOnRouteError(): any;
  export function useRouterOnRouteSuccess(): any;
  export function useRouterOnRouteStart(): any;
  export function useRouterOnRouteEnd(): any;
  export function useRouterOnRouteProgress(): any;
  export function useRouterOnRouteCancel(): any;
  export function useRouterOnRouteRedirect(): any;
  export function useRouterOnRouteResolve(): any;
  export function useRouterOnRouteUnresolve(): any;
  export function useRouterOnRouteMatch(): any;
  export function useRouterOnRouteUnmatch(): any;
  export function useRouterOnRouteGuard(): any;
  export function useRouterOnRouteGuardSuccess(): any;
  export function useRouterOnRouteGuardError(): any;
  export function useRouterOnRouteGuardCancel(): any;
  export function useRouterOnRouteGuardRedirect(): any;
  export function useRouterOnRouteGuardResolve(): any;
  export function useRouterOnRouteGuardUnresolve(): any;
  export function useRouterOnRouteGuardMatch(): any;
  export function useRouterOnRouteGuardUnmatch(): any;
}

// Phosphor Icons module declaration
declare module '@phosphor-icons/vue' {
  const PhosphorIcons: any;
  export default PhosphorIcons;
}

// Vue I18n module declarations
declare module 'vue-i18n' {
  export function useI18n(): { t: (key: string, values?: any) => string; locale: any };
  export function createI18n(options: any): any;
  export function useLocale(): any;
  export function useMessage(): any;
  export function useDateTimeFormat(): any;
  export function useNumberFormat(): any;
  export function usePluralization(): any;
  export function useTranslation(): any;
  export function useI18nRoute(): any;
  export function useI18nRouter(): any;
  export function useI18nLink(): any;
  export function useI18nMeta(): any;
  export function useI18nHead(): any;
  export function useI18nTitle(): any;
  export function useI18nError(): any;
  export function useI18nLoading(): any;
  export function useI18nReady(): any;
  export function useI18nComplete(): any;
  export function useI18nStart(): any;
  export function useI18nEnd(): any;
  export function useI18nProgress(): any;
  export function useI18nCancel(): any;
  export function useI18nRedirect(): any;
  export function useI18nResolve(): any;
  export function useI18nUnresolve(): any;
  export function useI18nMatch(): any;
  export function useI18nUnmatch(): any;
  export function useI18nGuard(): any;
  export function useI18nGuardSuccess(): any;
  export function useI18nGuardError(): any;
  export function useI18nGuardCancel(): any;
  export function useI18nGuardRedirect(): any;
  export function useI18nGuardResolve(): any;
  export function useI18nGuardUnresolve(): any;
  export function useI18nGuardMatch(): any;
  export function useI18nGuardUnmatch(): any;
}

declare module '*.vue' {
  import type { DefineComponent } from 'vue';
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>;
  export default component;
}

// Global window extensions
declare interface Window {
  showToast(message: string, type: 'success' | 'error' | 'info' | 'warning'): void;
  showConfirm(options: {
    title: string;
    message: string;
    confirmText: string;
    cancelText: string;
    isDanger: boolean;
  }): Promise<boolean>;
  showInput(options: {
    title: string;
    message: string;
    placeholder: string;
    confirmText: string;
    cancelText: string;
    suggestions: string[];
  }): Promise<string | null>;
  dispatchEvent(event: Event): boolean;
  addEventListener(type: string, listener: EventListenerOrEventListenerObject): void;
  removeEventListener(type: string, listener: EventListenerOrEventListenerObject): void;
}
