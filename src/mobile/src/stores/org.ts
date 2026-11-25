import { create } from 'zustand';

import type { paths } from '@/src/api/schema';

/** Shape of one org item returned by GET /v1/org */
export type OrgItem =
  paths['/v1/org']['get']['responses'][200]['content']['application/json']['organizations'][number];

export interface ActiveOrg {
  orgId: string;
  name: string;
  role: 'admin' | 'maker' | 'checker';
  checkerLevel: number;
  walletId: string;
  /** Balance in cents (integer, USD) */
  balance: number;
}

interface OrgState {
  /** null = personal wallet mode */
  activeOrg: ActiveOrg | null;
  switchOrg: (org: ActiveOrg) => void;
  exitOrg: () => void;
  /** Refresh balance after a transaction */
  updateBalance: (balance: number) => void;
}

export const useOrgStore = create<OrgState>((set) => ({
  activeOrg: null,
  switchOrg: (org) => set({ activeOrg: org }),
  exitOrg: () => set({ activeOrg: null }),
  updateBalance: (balance) =>
    set((state) =>
      state.activeOrg ? { activeOrg: { ...state.activeOrg, balance } } : state
    ),
}));

/** Synchronous accessor — use outside of React components. */
export function getActiveOrg(): ActiveOrg | null {
  return useOrgStore.getState().activeOrg;
}
