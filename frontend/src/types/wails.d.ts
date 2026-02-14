// Type declarations for Wails runtime
// This file provides type definitions for the Wails runtime API
// which may not be available in server mode

declare module '@wailsio/runtime' {
  export const Clipboard: {
    SetText: (text: string) => Promise<void>;
    GetText: () => Promise<string>;
  };

  export const EventsOn: (event: string, callback: (...args: any[]) => void) => void;
  export const EventsOff: (event: string, callback: (...args: any[]) => void) => void;
  export const EventsEmit: (event: string, ...args: any[]) => void;

  export const WindowGetCurrent: () => any;
  export const WindowShow: () => void;
  export const WindowHide: () => void;
  export const WindowMaximise: () => void;
  export const WindowToggleMaximise: () => void;
  export const WindowUnmaximise: () => void;
  export const WindowMinimise: () => void;
  export const WindowSetSystemDefaultTitle: (title: string) => void;
  export const WindowSetTitle: (title: string) => void;
  export const WindowClose: () => void;

  export const ScreenGetAll: () => any[];

  export const BrowserOpenURL: (url: string) => void;

  export const Environment: any;
}

// Extend Window interface to include wails property
declare global {
  interface Window {
    wails?: any;
  }
}
