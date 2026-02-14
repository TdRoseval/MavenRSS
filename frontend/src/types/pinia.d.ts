declare module 'pinia' {
  export function defineStore<Id extends string, S extends StateTree, G, A>(
    id: Id,
    storeSetup: () => S & G & ThisType<S & G & A>,
    options?: any
  ): StoreDefinition<Id, S, G, A>;

  export function createPinia(): any;

  export type StateTree = Record<string | number | symbol, any>;
  export interface StoreDefinition<Id, S, G, A> {
    (): { [P in keyof (S & G & A)]: (S & G & A)[P] };
  }
}
