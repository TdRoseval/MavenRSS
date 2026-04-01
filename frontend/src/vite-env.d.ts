/// <reference types="vite/client" />

// Keep this file minimal.
// Do not override core library types (`vue`, `vue-i18n`, `@phosphor-icons/vue`) here.

declare module '*.vue' {
  import type { DefineComponent } from 'vue';
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>;
  export default component;
}
