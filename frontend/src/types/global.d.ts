// Global type declarations

declare module 'pinia' {
  export function defineStore<Id extends string, S extends StateTree, G, A>(
    id: Id,
    storeSetup: () => S & G & ThisType<S & G & A>,
    options?: any
  ): StoreDefinition<Id, S, G, A>;

  export type StateTree = Record<string | number | symbol, any>;
  export interface StoreDefinition<Id, S, G, A> {
    (): { [P in keyof (S & G & A)]: (S & G & A)[P] };
  }
}

export interface ConfirmDialogOptions {
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  isDanger?: boolean;
}

export interface InputDialogOptions {
  title: string;
  message: string;
  placeholder?: string;
  defaultValue?: string;
  confirmText?: string;
  cancelText?: string;
  suggestions?: string[];
}

export interface MultiSelectOption {
  value: string;
  label: string;
  color?: string;
}

export interface MultiSelectDialogOptions {
  title: string;
  message: string;
  options: MultiSelectOption[];
  confirmText?: string;
  cancelText?: string;
}

export type ToastType = 'success' | 'error' | 'info' | 'warning';

declare global {
  interface Window {
    showConfirm: (ConfirmDialogOptions) => Promise<boolean>;
    showInput: (InputDialogOptions) => Promise<string | null>;
    showMultiSelect: (MultiSelectDialogOptions) => Promise<string[] | null>;
    showToast: (string, ToastType?, number?) => void;
  }

  // Browser APIs
  const confirm: (message: string) => boolean;
  const alert: (message: string) => void;
  const prompt: (message: string, defaultValue?: string) => string | null;
}

export {};
