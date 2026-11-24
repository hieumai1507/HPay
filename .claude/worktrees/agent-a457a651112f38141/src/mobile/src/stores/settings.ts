import { create } from 'zustand';

interface SettingsState {
  /**
   * Balance visibility — ALWAYS starts hidden on app open (doc 02 §5 fintech
   * default). Deliberately not persisted: persisting "visible" would defeat
   * the default-hidden rule on the next cold start.
   */
  balanceVisible: boolean;
  toggleBalanceVisible: () => void;
}

export const useSettingsStore = create<SettingsState>((set) => ({
  balanceVisible: false,
  toggleBalanceVisible: () => set((s) => ({ balanceVisible: !s.balanceVisible })),
}));
